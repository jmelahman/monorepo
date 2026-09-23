package rules

import (
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// PB3xx: code execution and obfuscation.
var execRules = []Rule{
	{
		ID:       "PB301",
		Name:     "top-level-execution",
		Severity: Critical,
		Doc: "Commands outside any function run the moment the PKGBUILD is sourced — including by " +
			"tools that only wanted metadata (`makepkg --printsrcinfo`, AUR helpers rendering a " +
			"preview). Top-level code should be limited to variable assignments.",
		Check: checkTopLevelExec,
	},
	{
		ID:       "PB302",
		Name:     "eval",
		Severity: Error, MaxSeverity: Critical, // critical when the evaluated string is downloaded
		Doc: "eval executes a string as code, defeating static review: what actually runs is " +
			"assembled at build time. Almost every legitimate use has a plain-bash equivalent.",
		Check: checkEval,
	},
	{
		ID:       "PB303",
		Name:     "decode-and-execute",
		Severity: Critical,
		Doc: "Decoding embedded data (base64, xxd, openssl enc) and piping it into an interpreter " +
			"is the canonical way to smuggle a payload past human review. There is no legitimate " +
			"reason for a PKGBUILD to execute decoded blobs.",
		Check: checkDecodeExec,
	},
	{
		ID:       "PB304",
		Name:     "download-and-execute",
		Severity: Critical,
		Doc: "Piping a download straight into an interpreter executes whatever the server chooses " +
			"to send, with no checksum, no review, and no record. This includes `eval \"$(curl ...)\"`, " +
			"`sh -c \"$(wget ...)\"` and `source <(curl ...)` variants.",
		Check: checkDownloadExec,
	},
	{
		ID:       "PB305",
		Name:     "dev-tcp",
		Severity: Critical,
		Doc: "/dev/tcp and /dev/udp redirections are bash's built-in network sockets — in a " +
			"PKGBUILD they are typically reverse shells or exfiltration, never packaging.",
		Check: checkDevTCP,
	},
	{
		ID:       "PB306",
		Name:     "unresolvable-command",
		Severity: Warn, MaxSeverity: Error, // error for ${!indirection}, which hides the name outright
		Doc: "A command whose name comes from indirection (${!var}) or command substitution cannot " +
			"be statically reviewed. In a PKGBUILD, hiding *which program runs* is itself a signal: " +
			"obfuscation is what this rule flags.",
		Check: checkDynamicCommands,
	},
	{
		ID:       "PB307",
		Name:     "obfuscated-payload",
		Severity: Warn,
		Doc: "Long hex-escape sequences or large base64-looking literals embedded in a build " +
			"script are how encoded payloads look at rest. Flagged for human review.",
		Check: checkObfuscatedLiterals,
	},
	{
		ID:       "PB308",
		Name:     "makepkg-function-override",
		Severity: Critical,
		Doc: "makepkg sources the PKGBUILD after defining its own internal functions, so a top-level " +
			"function that reuses an internal name (download_sources, verify_integrity_one, " +
			"create_package, …) silently replaces makepkg's implementation — a way to disable integrity " +
			"checks or tamper with fetching and packaging. Package functions are prepare/build/check/" +
			"package/package_*/pkgver only.",
		Check: checkMakepkgFuncOverride,
	},
	{
		ID:       "PB309",
		Name:     "hidden-unicode",
		Severity: Warn, MaxSeverity: Error, // error for bidi controls, which reorder what you read
		Doc: "Bidirectional-override and invisible/zero-width characters can make the rendered source " +
			"differ from what the shell executes (the \"Trojan Source\" class of attacks). A PKGBUILD is " +
			"ASCII shell; these controls have no legitimate place in it.",
		Check: checkHiddenUnicode,
	},
}

// criticalMakepkgFuncs are libmakepkg/makepkg internal functions whose behavior
// governs integrity verification, source fetching/extraction, or packaging.
// Redefining any of them at the top level of a PKGBUILD hijacks makepkg itself.
var criticalMakepkgFuncs = map[string]bool{
	// integrity & signature verification
	"check_checksums": true, "check_source_integrity": true, "check_pgpsigs": true,
	"check_option": true, "verify_file_signature": true, "verify_git_signature": true,
	"verify_integrity_one": true, "verify_integrity_sums": true, "source_has_signatures": true,
	// download
	"download_sources": true, "download_file": true, "download_git": true,
	"download_svn": true, "download_hg": true, "download_bzr": true,
	"download_fossil": true, "download_local": true,
	"get_downloadclient": true, "get_vcsclient": true,
	// source loading & extraction
	"extract_sources": true, "extract_file": true, "extract_git": true,
	"extract_svn": true, "extract_hg": true, "extract_bzr": true, "extract_fossil": true,
	"source_buildfile": true, "source_safe": true, "source_files": true,
	"source_makepkg_config": true,
	// packaging & phase runners
	"create_package": true, "create_package_signatures": true, "create_signature": true,
	"run_function": true, "run_function_safe": true, "run_pacman": true,
	"run_prepare": true, "run_build": true, "run_check": true, "run_verify": true,
	"run_package": true, "run_single_packaging": true, "run_split_packaging": true,
	"write_srcinfo": true,
}

func checkMakepkgFuncOverride(ctx *Context) []Finding {
	var out []Finding
	for name, fd := range ctx.Pkg.PKGBUILD.Functions {
		if criticalMakepkgFuncs[name] {
			out = append(out, findingAt("PB308", Critical, ctx.Pkg.PKGBUILD.Path, fd.Pos(),
				"top-level function %q shadows a makepkg internal of the same name, replacing its behavior", name))
		}
	}
	return out
}

// bidiControls are directional-override characters that can reorder how a line
// renders without changing what the shell executes.
var bidiControls = map[rune]string{
	0x202A: "U+202A LEFT-TO-RIGHT EMBEDDING", 0x202B: "U+202B RIGHT-TO-LEFT EMBEDDING",
	0x202C: "U+202C POP DIRECTIONAL FORMATTING", 0x202D: "U+202D LEFT-TO-RIGHT OVERRIDE",
	0x202E: "U+202E RIGHT-TO-LEFT OVERRIDE", 0x2066: "U+2066 LEFT-TO-RIGHT ISOLATE",
	0x2067: "U+2067 RIGHT-TO-LEFT ISOLATE", 0x2068: "U+2068 FIRST STRONG ISOLATE",
	0x2069: "U+2069 POP DIRECTIONAL ISOLATE",
}

// invisibleChars are zero-width or otherwise non-rendering characters that can
// hide text or split tokens invisibly.
var invisibleChars = map[rune]string{
	0x200B: "U+200B ZERO WIDTH SPACE", 0x200C: "U+200C ZERO WIDTH NON-JOINER",
	0x200D: "U+200D ZERO WIDTH JOINER", 0x2060: "U+2060 WORD JOINER",
	0xFEFF: "U+FEFF ZERO WIDTH NO-BREAK SPACE", 0x00AD: "U+00AD SOFT HYPHEN",
}

// hasNonASCII reports whether b contains any byte outside 7-bit ASCII, which
// is true of every byte of every multi-byte UTF-8 encoding.
func hasNonASCII(b []byte) bool {
	for _, c := range b {
		if c >= 0x80 {
			return true
		}
	}
	return false
}

func checkHiddenUnicode(ctx *Context) []Finding {
	var out []Finding
	units := ctx.Pkg.Units()
	for i := range units {
		u := &units[i]
		// Every rune in both tables is above U+007F, so it encodes to bytes
		// that all have the high bit set. A unit with no such byte cannot
		// contain one, and almost no PKGBUILD does — which makes this scan
		// worth doing before splitting the file into lines and decoding each
		// one twice.
		if !hasNonASCII(u.Raw) {
			continue
		}
		for lineNo, line := range strings.Split(string(u.Raw), "\n") {
			for _, r := range line {
				if name, ok := bidiControls[r]; ok {
					out = append(out, Finding{RuleID: "PB309", Severity: Error,
						Message: "bidirectional control character " + name + " can hide what the shell runs",
						Path:    u.Path, Line: lineNo + 1, Col: 1})
					break
				}
			}
			for _, r := range line {
				if name, ok := invisibleChars[r]; ok {
					out = append(out, Finding{RuleID: "PB309", Severity: Warn,
						Message: "invisible character " + name + " embedded in source",
						Path:    u.Path, Line: lineNo + 1, Col: 1})
					break
				}
			}
		}
	}
	return out
}

// Top-level builtins that only manipulate the shell's own state: its
// variables, its options, its current directory, and where control goes next.
// None of them opens a file, spawns a process, or reaches the network, so a
// PKGBUILD sourced for its metadata is no worse off for having run one.
var benignTopLevel = map[string]bool{
	"unset": true, "export": true, "declare": true, "typeset": true, "local": true,
	"readonly": true, "set": true, "shopt": true, ":": true, "true": true, "false": true,
	// Control flow. `exit` ends the sourcing shell rather than running
	// anything, which is what a guard like `_die() { error "$@"; exit 1; }`
	// exists to do; scoring it alongside a downloader said the two were the
	// same kind of problem.
	"exit": true, "return": true, "break": true, "continue": true, "shift": true,
	// Variable binding from input the caller already chose: a herestring, a
	// process substitution, a redirect. `IFS=. read -r _major _minor <<< "$pkgver"`
	// is the portable way to split a version at the top level.
	"read": true, "mapfile": true, "readarray": true, "getopts": true,
	// Shell state the rest of the file reads back: the umask new files get,
	// the directory relative paths resolve against, the command-lookup cache.
	"umask": true, "cd": true, "pushd": true, "popd": true, "hash": true,
}

// pureTopLevel are commands whose whole effect is to test a condition or write
// to stdout/stderr: they open no file, spawn no interpreter, and reach no
// network. Arch and build-flag guards (`if [ "$CARCH" = i686 ]` selecting a
// depends+=()) and status banners are the bulk of what this rule used to
// report, and scoring them alongside an unauthenticated `curl | sh` buried the
// findings worth acting on.
//
// `[` and `test` also settle an asymmetry: `[[ ]]` and `(( ))` parse as
// TestClause and ArithmCmd rather than CallExpr, so they never reached this
// rule at all. Flagging one spelling of a condition and not the other meant a
// stylistic rewrite could move a package from F to A.
var pureTopLevel = map[string]bool{
	"[": true, "test": true,
	"echo": true, "printf": true, "cat": true, "tput": true,
	// A pure reader: grep has no write form at all, and `grep -q` guards are
	// how PKGBUILDs probe the host for a feature before appending a depends.
	"grep": true, "egrep": true, "fgrep": true,
	// Stream filters with no output-file option at all: what they are handed
	// on stdin comes back on stdout, and nothing else moves. `pkgver="$(echo
	// $_v | tr . $'\n' | tac | paste -s -d .)"` is four of them in a row
	// computing a string, which the rule used to score as four criticals.
	"cut": true, "tr": true, "rev": true, "tac": true, "paste": true,
	"head": true, "tail": true, "nl": true, "fold": true, "wc": true,
	"expand": true, "unexpand": true, "comm": true, "join": true, "column": true,
	"seq": true, "expr": true, "diff": true, "cmp": true, "od": true, "strings": true,
	// A JSON filter with no way to write a file or run a program: jq's only
	// outputs are stdout and its exit status.
	"jq": true,
	// String surgery on a path. None of them touches the filesystem beyond
	// resolving it, and realpath/readlink have no write form.
	"basename": true, "dirname": true, "realpath": true, "readlink": true, "pwd": true,
	// Reading the host to decide what to declare: how many cores, which user,
	// what the C library says. Like uname below, these are the inputs a
	// PKGBUILD's top-level `depends+=()` guards are written against.
	"nproc": true, "arch": true, "getconf": true, "hostname": true,
	"whoami": true, "id": true, "logname": true, "tty": true, "getent": true,
	"ls": true, "stat": true, "file": true, "df": true, "ps": true,
	// Where the user's XDG directories point. It reads user-dirs.dirs and
	// prints one line; there is no form that writes one.
	"xdg-user-dir": true,
	// Where would this name resolve? A lookup runs nothing. `command -v` is
	// resolved to `command` in newCommand so it never arrives here as its
	// argument.
	"command": true, "type": true, "which": true, "whereis": true,
	// Digests read their input and print a hash; none of them writes.
	"md5sum": true, "sha1sum": true, "sha224sum": true, "sha256sum": true,
	"sha384sum": true, "sha512sum": true, "b2sum": true, "cksum": true,
	// pacman's version comparator: two arguments in, an ordering out.
	"vercmp": true,
	// Prints the host's identity and exits. Every option (-a -s -n -r -v -m
	// -p -i -o) reads, so unlike date and sed there is no escape hatch to
	// judge per call — coreutils uname has no --set and no operand form.
	// PB901 already treats `case $(uname -m)` as *the* portable arch-dispatch
	// idiom (see dispatchesOnArch); rating that same line Critical here left
	// the two rules disagreeing about one statement.
	"uname": true,
	// libmakepkg's message API, sourced into the PKGBUILD's shell before it
	// runs. PB907 already rates these a portability nit rather than a hazard.
	"msg": true, "msg2": true, "warning": true, "error": true, "plain": true,
	// libmakepkg query helpers, sourced the same way: check_option reads the
	// OPTIONS array and in_array searches one. Both answer a question about
	// what makepkg was already told.
	"check_option": true, "in_array": true,
}

// pureTopLevelUse reports whether this invocation is one of the pure ones.
// The names below are pure in the form PKGBUILDs use them but each keeps an
// escape hatch into the filesystem, the network, or another program, so they
// are judged per call rather than by name.
func pureTopLevelUse(c Command) bool {
	if pureTopLevel[c.Name] {
		return true
	}
	switch c.Name {
	case "date":
		// `export KBUILD_BUILD_TIMESTAMP="$(date -Ru…)"` is the kernel
		// packages' reproducible-builds idiom, verbatim from core/linux.
		// Reading the clock is pure; setting it is not.
		return !dateSetsClock(c)
	case "sed":
		// Filtering a pipe is pure; -i and the w script forms write files.
		return !sedInPlace(c) && !sedWritesFile(c)
	case "sort":
		// -o writes the sorted result to a file, and --compress-program names
		// a program sort runs over its temporaries.
		return !hasFlag(c, "-o", "--output") && !hasFlag(c, "--compress-program")
	case "uniq":
		// `uniq in out` writes its second operand; every other form filters.
		return !uniqWritesFile(c)
	case "iconv":
		return !hasFlag(c, "-o", "--output")
	case "find":
		return !findActs(c)
	case "awk", "gawk", "mawk", "nawk":
		return awkOnlyFilters(c)
	case "pacman":
		return pacmanQueries(c)
	case "pkgfile":
		// The local file database is a read; -u refreshes it over the network.
		return !hasFlag(c, "-u", "--update")
	case "xargs":
		return xargsRunsPure(c)
	}
	return false
}

// xargsValueFlags are the xargs options that require a value, so the word after
// them is not the command xargs runs. The optional-value ones (-e, -i, -l,
// --eof, --replace, --max-lines) are deliberately absent: their value has to be
// attached, which makes the next word the command — skipping it would let
// `xargs -i rm` read as a bare `xargs`.
var xargsValueFlags = map[string]bool{
	"-a": true, "-d": true, "-E": true, "-I": true,
	"-L": true, "-n": true, "-P": true, "-s": true,
	"--arg-file": true, "--delimiter": true, "--max-args": true,
	"--max-procs": true, "--max-chars": true, "--process-slot-var": true,
}

// xargsRunsPure reports whether the program xargs would run is one of the
// unconditionally pure ones. Trailing `| xargs` to trim whitespace, and
// `| xargs -I@ printf …` to reshape a list into source= entries, are how
// PKGBUILDs assemble arrays at the top level; what makes them safe is the
// command on the end, not xargs itself.
func xargsRunsPure(c Command) bool {
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		switch {
		case a == "--":
			// Everything after is the command and its arguments.
			return i+1 < len(c.Args) && pureTopLevel[c.Args[i+1]] && !c.ArgDyn[i+1]
		case len(a) > 1 && a[0] == '-':
			name := a
			if eq := strings.IndexByte(a, '='); eq > 0 && strings.HasPrefix(a, "--") {
				// The value came attached — `--max-args=1 rm` — so the next
				// word is the command, not the option's argument.
				name = ""
			} else if a[1] != '-' && len(a) > 2 {
				// A short option with its value attached, `-n1`, or a cluster.
				// Either way the value is not a separate word.
				name = ""
			}
			if xargsValueFlags[name] {
				i++
			}
		default:
			// The first operand is the command; with no operand xargs runs
			// echo, which prints and nothing else.
			return pureTopLevel[a] && !c.ArgDyn[i]
		}
	}
	return true
}

// hasFlag reports whether the command passes any of the given options, in the
// spellings that carry them: exactly (`-o`, `--output`), attached (`-ofile`,
// `--output=file`), or bundled into a short cluster (`-uo`). Long options are
// matched whole; short ones by their letter, since a cluster does not say
// where one option ends.
func hasFlag(c Command, opts ...string) bool {
	for _, a := range c.Args {
		if len(a) < 2 || a[0] != '-' {
			continue
		}
		long := a[1] == '-'
		name, _, _ := strings.Cut(a, "=")
		for _, opt := range opts {
			switch {
			case strings.HasPrefix(opt, "--"):
				if long && name == opt {
					return true
				}
			case !long && strings.ContainsRune(a[1:], rune(opt[1])):
				return true
			}
		}
	}
	return false
}

// uniqValueFlags are the uniq options that take a separate value, so that the
// value is not miscounted as the operand that would make this a write.
var uniqValueFlags = map[string]bool{
	"-f": true, "-s": true, "-w": true,
	"--skip-fields": true, "--skip-chars": true, "--check-chars": true,
}

func uniqWritesFile(c Command) bool {
	operands := 0
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		if uniqValueFlags[a] {
			i++
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			continue
		}
		operands++
	}
	return operands >= 2
}

// findActs reports whether a find invocation does more than name what it
// matched: -exec and friends run a command, -delete removes files, and the
// -f* actions write their output to a file find is given.
func findActs(c Command) bool {
	for _, a := range c.Args {
		switch a {
		case "-exec", "-execdir", "-ok", "-okdir", "-delete",
			"-fls", "-fprint", "-fprint0", "-fprintf":
			return true
		}
	}
	return false
}

// awkEscapesRe matches the awk constructs that leave the program: a redirect
// into a file (`print > "out"`), a pipe into a shell command (`print | "sh"`),
// system(), and gawk's extension loader.
//
// Requiring a quote after the redirect is what keeps the two idioms that read
// like one apart: `while (getline > 0)` and `if (level > 0)` are comparisons,
// and an awk that reads /proc/cpuinfo to pick an x86-64 feature level is the
// most common awk in the corpus.
var awkEscapesRe = regexp.MustCompile(`system[[:space:]]*\(|>>?[[:space:]]*["']|(^|[^|])\|&?[[:space:]]*["']|@load`)

// awkOnlyFilters reports whether an awk invocation only reads its input and
// prints. A program pkglint cannot read — assembled from an expansion, or in a
// separate file named by -f — is not judged pure: what it does is not in front
// of the reviewer either.
func awkOnlyFilters(c Command) bool {
	prog := -1
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		switch {
		case a == "-f" || a == "--file" || strings.HasPrefix(a, "-f") ||
			strings.HasPrefix(a, "--file="),
			// gawk's other ways to run what is not the positional program:
			// program text inside an option (--source), a program or library
			// file (-i/--include, -E/--exec), a compiled extension loaded by
			// name (-l/--load). `-i inplace` also rewrites its input files.
			strings.HasPrefix(a, "-i"), strings.HasPrefix(a, "-l"),
			strings.HasPrefix(a, "-E"), strings.HasPrefix(a, "--source"),
			strings.HasPrefix(a, "--include"), strings.HasPrefix(a, "--load"),
			strings.HasPrefix(a, "--exec"):
			return false
		case a == "-v" || a == "-F" || a == "--assign" || a == "--field-separator":
			i++ // a detached option value, not the program
		case len(a) > 1 && a[0] == '-':
		default:
			prog = i
		}
		if prog >= 0 {
			break
		}
	}
	// The rendered text cannot answer this: awk's own `$1` and a shell `$_prog`
	// that named no known variable both come back as "$name", so the program is
	// judged from the word it was written as. A single-quoted awk program holds
	// no expansions at all, which is the form the corpus writes.
	if prog < 0 || prog >= len(c.ArgWord) || !wordIsLiteral(c.ArgWord[prog]) {
		return false
	}
	return !awkEscapesRe.MatchString(c.Args[prog])
}

// wordIsLiteral reports whether a word is spelled out in full: only literal
// text, in quotes or out. Any expansion — a parameter, a command substitution,
// arithmetic, a process substitution — means the value the command actually
// receives is not the one in front of the reviewer.
func wordIsLiteral(w *syntax.Word) bool {
	if w == nil {
		return false
	}
	literal := true
	syntax.Walk(w, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.Word, *syntax.Lit, *syntax.SglQuoted, *syntax.DblQuoted:
		case nil:
		default:
			literal = false
		}
		return literal
	})
	return literal
}

// pacmanQueries reports whether a pacman invocation only reads the databases.
// -Q and -T answer questions about what is installed, and a -S asked to
// --print says what it would do without doing it. Everything else either
// changes the system or, with -y/-u, refreshes over the network.
func pacmanQueries(c Command) bool {
	var reads, writes, refreshes, prints bool
	for _, a := range c.Args {
		if len(a) < 2 || a[0] != '-' {
			continue
		}
		if a[1] == '-' {
			name, _, _ := strings.Cut(a, "=")
			switch name {
			case "--query", "--deptest", "--files", "--version", "--help":
				reads = true
			case "--upgrade", "--remove", "--database":
				writes = true
			case "--refresh", "--sysupgrade":
				refreshes = true
			case "--print", "--print-format":
				prints = true
			}
			continue
		}
		for _, r := range a[1:] {
			switch r {
			case 'Q', 'T', 'F', 'V', 'h':
				reads = true
			case 'U', 'R', 'D':
				writes = true
			case 'y', 'u':
				refreshes = true
			case 'p':
				prints = true
			}
		}
	}
	if writes || refreshes {
		return false
	}
	return reads || prints
}

// dateSetsClock reports whether this date invocation sets the system clock
// rather than printing it: GNU -s/--set, or the POSIX operand form — a
// positional argument not starting with '+' (`date 0501120026`).
func dateSetsClock(c Command) bool {
	for i := 0; i < len(c.Args); i++ {
		a := c.Args[i]
		switch {
		case strings.HasPrefix(a, "--"):
			if strings.HasPrefix(a, "--set") {
				return true
			}
			// A detached value follows its option; don't read it as the
			// POSIX operand form.
			if a == "--date" || a == "--file" || a == "--reference" {
				i++
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			if strings.ContainsRune(a[1:], 's') {
				return true // -s anywhere in a cluster is --set
			}
			// A trailing -d/-f/-r takes the next word as its value; attached
			// values (-d@1234) are already part of this argument.
			switch a[len(a)-1] {
			case 'd', 'f', 'r':
				i++
			}
		case !strings.HasPrefix(a, "+"):
			return true // POSIX operand form sets the clock
		}
	}
	return false
}

// sedWriteRe conservatively matches sed's file-writing script forms: the
// `w file` / `W file` command (possibly address-prefixed) and the `s///w file`
// flag. A replacement that merely contains " w " keeps the finding — for this
// rule that is the right direction to be wrong.
var sedWriteRe = regexp.MustCompile(`(^|[;{[:space:]/0-9$])[wW][[:space:]]`)

func sedWritesFile(c Command) bool {
	for _, a := range c.Args {
		// Telling script arguments from input files apart needs full option
		// parsing; checking every non-flag argument over-matches at worst.
		if !strings.HasPrefix(a, "-") && sedWriteRe.MatchString(a) {
			return true
		}
	}
	return false
}

// discardSinks are redirection targets that create nothing: the bit bucket and
// the streams the shell already holds open. `>/dev/null` is how a PKGBUILD
// silences a probe — `type msg >/dev/null 2>&1`, `getent group nobody
// >/dev/null || _gid=30` — and reading it as a write made the quiet spelling of
// an inert guard Critical while the noisy one passed.
var discardSinks = map[string]bool{
	"/dev/null": true, "/dev/stdout": true, "/dev/stderr": true,
}

func discardsOutput(target string) bool {
	return discardSinks[target] || strings.HasPrefix(target, "/dev/fd/")
}

// redirectsOutput reports whether the statement sends its output to a file,
// which is what separates a banner from a write: `cat <<EOF > helper.sh` at
// the top level drops a file on disk the moment the PKGBUILD is sourced.
func redirectsOutput(stmt *syntax.Stmt) bool {
	for _, r := range stmt.Redirs {
		switch r.Op {
		case syntax.RdrOut, syntax.AppOut, syntax.RdrAll, syntax.AppAll:
			if target, _ := renderPlain(r.Word); discardsOutput(target) {
				continue
			}
			return true
		}
	}
	return false
}

// inert reports whether running c has no effect a PKGBUILD sourced for its
// metadata would care about: it opens no file for writing, starts no
// interpreter, and reaches no network. pure carries the verdict for the file's
// own functions, so a call to a helper is judged by what the helper does.
func (ctx *Context) inert(c Command, pure map[string]bool) bool {
	if c.Name == "" {
		return false // a name assembled at run time is not reviewable
	}
	// A PKGBUILD that supplies the body decides what the name means: `warning`
	// is libmakepkg's banner right up until the file writes one of its own.
	if ctx.definesFunc(c, c.Name) {
		return pure[c.Unit.Path+"\x00"+c.Name]
	}
	return benignTopLevel[c.Name] || pureTopLevelUse(c)
}

// funcPurity returns, for every function the file declares, whether calling it
// does anything beyond reading, testing and assigning. Helper functions are how
// PKGBUILDs keep the top level readable — `_is_lto_kernel` is a `[[ ]]` and a
// `return`, `_arch_map` is a `case` printing a string, `_source_main` is two
// array assignments — and judging a call by its name alone made the tidier
// spelling of a top-level `if` the more severely graded one.
//
// It is a greatest fixpoint: assume every declared function is pure, then drop
// any whose body holds something that is not, and repeat until a pass changes
// nothing. Two helpers that call each other come out pure, which a depth-first
// walk would need cycle detection to get right, while a body that reaches a
// downloader through any number of hops drops every caller along the way.
func (ctx *Context) funcPurity() map[string]bool {
	if ctx.pureFn != nil {
		return ctx.pureFn
	}
	ctx.pureFn = map[string]bool{}
	if len(ctx.funcDecls) == 0 {
		return ctx.pureFn
	}
	byFn := make(map[string][]Command, len(ctx.funcDecls))
	for _, c := range ctx.cmds {
		if c.Fn == "" {
			continue
		}
		key := c.Unit.Path + "\x00" + c.Fn
		byFn[key] = append(byFn[key], c)
	}
	pure := ctx.pureFn
	for key := range ctx.funcDecls {
		pure[key] = true
	}
	for changed := true; changed; {
		changed = false
		for key, fd := range ctx.funcDecls {
			if !pure[key] || ctx.bodyInert(byFn[key], fd, pure) {
				continue
			}
			pure[key] = false
			changed = true
		}
	}
	return pure
}

func (ctx *Context) bodyInert(cmds []Command, fd *syntax.FuncDecl, pure map[string]bool) bool {
	for _, c := range cmds {
		if !ctx.inert(c, pure) {
			return false
		}
	}
	// A redirect can hang off a compound statement rather than any one command
	// — `{ …; } > helper.sh`, a whole `for` loop appending to a file — so the
	// body is walked for writes instead of asking each command about its own.
	inert := true
	syntax.Walk(fd.Body, func(n syntax.Node) bool {
		if stmt, ok := n.(*syntax.Stmt); ok && redirectsOutput(stmt) {
			inert = false
		}
		return inert
	})
	return inert
}

func checkTopLevelExec(ctx *Context) []Finding {
	var out []Finding
	pure := ctx.funcPurity()
	for _, c := range ctx.Commands() {
		if c.Fn != "" || c.Unit.Scriptlet {
			continue
		}
		// eval belongs to PB302, which reports it wherever it appears and
		// escalates to Critical on its own when the evaluated string was
		// downloaded. Reporting it here as well stacks a second Critical on
		// the identical statement without adding anything a reader could act
		// on differently.
		if c.Name == "eval" {
			continue
		}
		if ctx.inert(c, pure) && !redirectsOutput(c.Stmt) {
			continue
		}
		name := c.Name
		if name == "" {
			name = c.RawName
		}
		out = append(out, c.finding("PB301", Critical,
			"%q executes at PKGBUILD top level — it runs whenever the file is sourced, even for metadata extraction", name))
	}
	return out
}

var shellSinks = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true, "ksh": true,
	"python": true, "python3": true, "python2": true, "perl": true, "ruby": true, "node": true,
}

var downloaders = map[string]bool{"curl": true, "wget": true, "fetch": true, "aria2c": true, "axel": true}

// scriptInterpreters are the shellSinks that run a program named on their own
// command line. A shell executes stdin by default — `curl … | sh` runs what it
// downloaded — but `python3 -c '<program>'` and `node parse.js` were handed a
// program that is already on disk and under review, so the piped response is
// that program's *input*, not code. The AUR idiom this distinguishes is the
// pkgver() helper that fetches a registry's JSON and prints one field.
var scriptInterpreters = map[string]bool{
	"python": true, "python3": true, "python2": true,
	"perl": true, "ruby": true, "node": true,
}

// sinkExecutesStdin reports whether a pipeline's terminating interpreter treats
// what it reads as code. Shells always do. A scriptInterpreter only does so
// when given no program of its own, or an explicit "-" naming stdin.
func sinkExecutesStdin(c Command) bool {
	if !scriptInterpreters[c.Name] {
		return true // a shell: stdin is the script
	}
	for _, a := range c.Args {
		if a == "-" {
			return true // read the program from stdin
		}
		if strings.HasPrefix(a, "-") {
			continue // a flag; -c/-e/-m carry their program in the next word
		}
		// A program to run, so stdin is normally its input — unless the
		// program itself hands stdin to an evaluator, which is the same
		// download-and-execute with one more hop. Matching the literal is
		// conservative on purpose: over-reporting here costs a review, and
		// under-reporting costs the finding this rule exists for.
		return stdinEvalRe.MatchString(a)
	}
	return true // bare interpreter: stdin is the script
}

// stdinEvalRe matches the evaluator names an inline interpreter program would
// use to execute what it read, across the scriptInterpreters' languages.
var stdinEvalRe = regexp.MustCompile(`\b(exec|eval|compile|system|instance_eval|Function)\b`)

var decoders = map[string]bool{"base64": true, "base32": true, "xxd": true, "uudecode": true, "openssl": true}

func checkEval(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if c.Name != "eval" || inertEval(c) {
			continue
		}
		sev := Error
		msg := "eval assembles and executes code at build time, defeating static review"
		for _, w := range c.Call.Args[1:] {
			if wordContainsCommand(w, downloaders) {
				sev = Critical
				msg = "eval executes the output of a network download"
			}
		}
		out = append(out, c.finding("PB302", sev, "%s", msg))
	}
	return out
}

// evalMetadataVars are the package-metadata arrays an inert eval may assign.
// Maintainers eval a conditional `depends+=(...)` in package() to keep it out
// of the metadata makepkg scrapes for .SRCINFO; the string is constant, so
// review sees exactly what runs. Nothing here is executed or sourced:
// `install` names a scriptlet and the source/checksum arrays feed downloads,
// so they stay out.
var evalMetadataVars = map[string]bool{
	"depends": true, "optdepends": true, "makedepends": true, "checkdepends": true,
	"provides": true, "conflicts": true, "replaces": true, "backup": true, "groups": true,
}

// inertEval reports whether an eval's string is a constant that parses to
// nothing but literal assignments to package-metadata arrays — code a reviewer
// reads in full, which runs no command and expands nothing. Anything else,
// including a string pkglint cannot render exactly, stays PB302's.
func inertEval(c Command) bool {
	var src strings.Builder
	for i, w := range c.Call.Args[1:] {
		if i > 0 {
			src.WriteByte(' ')
		}
		lit, ok := literalWord(w, false)
		if !ok {
			return false
		}
		src.WriteString(lit)
	}
	f, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(src.String()), "")
	if err != nil || len(f.Stmts) == 0 {
		return false
	}
	for _, st := range f.Stmts {
		call, ok := st.Cmd.(*syntax.CallExpr)
		if !ok || len(call.Args) > 0 || len(st.Redirs) > 0 || st.Negated || st.Background || st.Coprocess {
			return false
		}
		for _, as := range call.Assigns {
			if as.Name == nil || as.Index != nil || as.Naked || !evalMetadataVars[metadataBase(as.Name.Value)] {
				return false
			}
			if as.Value != nil {
				if _, ok := literalWord(as.Value, true); !ok {
					return false
				}
			}
			if as.Array != nil {
				for _, el := range as.Array.Elems {
					if el.Index != nil || el.Value == nil {
						return false
					}
					if _, ok := literalWord(el.Value, true); !ok {
						return false
					}
				}
			}
		}
	}
	return true
}

// metadataBase strips an architecture suffix: depends_x86_64 is depends. No
// metadata name contains an underscore, so the first one starts the suffix.
func metadataBase(name string) string {
	base, _, _ := strings.Cut(name, "_")
	return base
}

// literalWord renders a word that expands nothing — plain text, single quotes,
// double quotes around plain text — returning its value.
//
// For eval's own arguments (inner false) the value is the text eval will
// parse, so a backslash in unquoted or double-quoted text is refused rather
// than unescaped: exactness matters more than coverage. Single-quoted text is
// already exact. Inside the evaluated string (inner true) the value is never
// used, only vetted, so escapes are harmless; but an unquoted word there is
// subject to globbing, brace and tilde expansion, so `depends=(*.so)` would
// take its contents from the build directory rather than from the text.
func literalWord(w *syntax.Word, inner bool) (string, bool) {
	var b strings.Builder
	lit := func(v string) bool {
		if !inner && strings.ContainsRune(v, '\\') {
			return false
		}
		b.WriteString(v)
		return true
	}
	for i, p := range w.Parts {
		switch p := p.(type) {
		case *syntax.Lit:
			if inner && (strings.ContainsAny(p.Value, "*?[{") || (i == 0 && strings.HasPrefix(p.Value, "~"))) {
				return "", false
			}
			if !lit(p.Value) {
				return "", false
			}
		case *syntax.SglQuoted:
			if p.Dollar {
				return "", false
			}
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			if p.Dollar {
				return "", false
			}
			for _, q := range p.Parts {
				l, ok := q.(*syntax.Lit)
				if !ok || !lit(l.Value) {
					return "", false
				}
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

func checkDecodeExec(ctx *Context) []Finding {
	return ctx.checkPipeInto(decoders, func(c Command) bool {
		// openssl only decodes with `enc -d` / base64 -d style flags.
		if c.Name == "openssl" {
			return c.Subcommand() == "enc" || c.Subcommand() == "base64"
		}
		if c.Name == "base64" || c.Name == "base32" {
			return c.HasArg("-d") || c.HasArg("--decode") || c.HasArg("-D")
		}
		if c.Name == "xxd" {
			return c.HasArg("-r")
		}
		return true
	}, "PB303", "decoded data is piped into %s: embedded payload execution")
}

func checkDownloadExec(ctx *Context) []Finding {
	out := ctx.checkPipeInto(downloaders, func(Command) bool { return true },
		"PB304", "a network download is piped straight into %s and executed")

	// eval "$(curl ...)" is handled by PB302; here: sh -c "$(curl ...)" and
	// source <(curl ...).
	for _, c := range ctx.Commands() {
		if shellSinks[c.Name] || c.Name == "source" || c.Name == "." {
			for _, w := range c.Call.Args[1:] {
				if wordContainsCommand(w, downloaders) {
					out = append(out, c.finding("PB304", Critical,
						"%s executes the output of a network download", c.Name))
					break
				}
			}
		}
	}
	return out
}

// checkPipeInto flags pipelines where a command from sources (passing
// qualify) ends up in an interpreter sink.
func (ctx *Context) checkPipeInto(sources map[string]bool, qualify func(Command) bool, id, format string) []Finding {
	var out []Finding
	units := ctx.Pkg.Units()
	for i := range units {
		u := &units[i]
		for _, segs := range pipelines(u.File) {
			last := segs[len(segs)-1]
			sinkName := stmtCommandName(last, ctx.vars)
			if !shellSinks[sinkName] {
				continue
			}
			if call, ok := last.Cmd.(*syntax.CallExpr); ok &&
				!sinkExecutesStdin(ctx.newCommand(u, "", last, call)) {
				continue
			}
			sink := sinkName
			for _, seg := range segs[:len(segs)-1] {
				call, ok := seg.Cmd.(*syntax.CallExpr)
				if !ok || len(call.Args) == 0 {
					continue
				}
				c := ctx.newCommand(u, "", seg, call)
				if sources[c.Name] && qualify(c) {
					out = append(out, findingAt(id, Critical, u.Path, seg.Pos(), format, sink))
					break
				}
			}
		}
	}
	return out
}

func checkDevTCP(ctx *Context) []Finding {
	var out []Finding
	units := ctx.Pkg.Units()
	for i := range units {
		u := &units[i]
		syntax.Walk(u.File, func(node syntax.Node) bool {
			r, ok := node.(*syntax.Redirect)
			if !ok {
				return true
			}
			if r.Word == nil {
				return true
			}
			s, _ := renderPlain(r.Word)
			if strings.Contains(s, "/dev/tcp/") || strings.Contains(s, "/dev/udp/") {
				out = append(out, findingAt("PB305", Critical, u.Path, r.Pos(),
					"raw %s socket redirection", s))
			}
			return true
		})
	}
	return out
}

func checkDynamicCommands(ctx *Context) []Finding {
	var out []Finding
	for _, c := range ctx.Commands() {
		if c.Name != "" || len(c.Call.Args) == 0 {
			continue
		}
		first := c.Call.Args[0]
		indirect := false
		syntax.Walk(first, func(node syntax.Node) bool {
			if pe, ok := node.(*syntax.ParamExp); ok && pe.Excl {
				indirect = true
			}
			return true
		})
		switch {
		case indirect:
			out = append(out, c.finding("PB306", Error,
				"command name via ${!indirection}: what runs cannot be determined statically"))
		case c.Dynamic && !inBuildTreePath(c.RawName):
			out = append(out, c.finding("PB306", Warn,
				"command name %q is not statically resolvable", c.RawName))
		}
	}
	return out
}

// inBuildTreePath reports whether a command path is a file under $srcdir,
// $pkgdir or $startdir: `"$srcdir/foo-${arch[0]}.AppImage" --appimage-extract`
// runs something the sources put there, whichever variant it picks.
func inBuildTreePath(name string) bool {
	for _, root := range []string{"$srcdir/", "${srcdir}/", "$pkgdir/", "${pkgdir}/", "$startdir/", "${startdir}/"} {
		if rest, ok := strings.CutPrefix(name, root); ok {
			return !strings.Contains(rest, "..")
		}
	}
	return false
}

var (
	hexRunRe     = regexp.MustCompile(`(\\x[0-9a-fA-F]{2}){8,}`)
	base64BlobRe = regexp.MustCompile(`[A-Za-z0-9+/]{120,}={0,2}`)
)

func checkObfuscatedLiterals(ctx *Context) []Finding {
	var out []Finding
	units := ctx.Pkg.Units()
	for i := range units {
		u := &units[i]
		for lineNo, line := range strings.Split(string(u.Raw), "\n") {
			if slices.ContainsFunc(hexRunRe.FindAllString(line, -1), variedBytes) {
				out = append(out, Finding{RuleID: "PB307", Severity: Warn,
					Message: "long hex-escape run looks like an encoded payload",
					Path:    u.Path, Line: lineNo + 1, Col: 1})
			}
			if m := base64BlobRe.FindString(line); m != "" && !looksLikeDigestContext(line) {
				out = append(out, Finding{RuleID: "PB307", Severity: Warn,
					Message: "large base64-like literal embedded in build script",
					Path:    u.Path, Line: lineNo + 1, Col: 1})
			}
		}
	}
	return out
}

// variedBytes reports whether a \xNN run spells more than one byte value. A
// run of one repeated byte is padding (epsonscan2 NUL-pads a path it patches
// into a binary with bbe), not an encoded payload.
func variedBytes(run string) bool {
	run = strings.ToLower(run)
	return strings.Count(run, run[:4]) != len(run)/4
}

// looksLikeDigestContext avoids flagging checksum arrays: hex digests are
// subsets of the base64 alphabet.
func looksLikeDigestContext(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.Contains(trimmed, "sums") {
		return true
	}
	m := base64BlobRe.FindString(line)
	return regexp.MustCompile(`^[0-9a-fA-F]+$`).MatchString(m)
}
