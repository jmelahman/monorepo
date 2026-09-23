package pkgfile

import (
	"strconv"
	"strings"
)

// PkgInfo is the parsed .PKGINFO metadata makepkg writes into every package.
type PkgInfo struct {
	Name     string
	Base     string
	Version  string
	Desc     string
	URL      string
	Arch     string
	Packager string
	Size     int64

	License     []string
	Depends     []string
	OptDepends  []string
	MakeDepends []string
	Provides    []string
	Conflicts   []string
	Replaces    []string
	Backup      []string
	Groups      []string
	XData       []string
}

// IsDebug reports whether this is a -debug split package (which ships
// intentionally unstripped, oddly-placed ELF files).
func (i PkgInfo) IsDebug() bool {
	if !strings.HasSuffix(i.Name, "-debug") {
		return false
	}
	for _, x := range i.XData {
		if x == "pkgtype=debug" {
			return true
		}
	}
	// Older packages carry no xdata; fall back to the name convention.
	return true
}

// parsePkgInfo reads makepkg's "key = value" format. Repeated keys append.
func parsePkgInfo(data []byte) PkgInfo {
	var info PkgInfo
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "pkgname":
			info.Name = val
		case "pkgbase":
			info.Base = val
		case "pkgver":
			info.Version = val
		case "pkgdesc":
			info.Desc = val
		case "url":
			info.URL = val
		case "arch":
			info.Arch = val
		case "packager":
			info.Packager = val
		case "size":
			info.Size, _ = strconv.ParseInt(val, 10, 64)
		case "license":
			info.License = append(info.License, val)
		case "depend":
			info.Depends = append(info.Depends, val)
		case "optdepend":
			info.OptDepends = append(info.OptDepends, val)
		case "makedepend":
			info.MakeDepends = append(info.MakeDepends, val)
		case "provides":
			info.Provides = append(info.Provides, val)
		case "conflict":
			info.Conflicts = append(info.Conflicts, val)
		case "replaces":
			info.Replaces = append(info.Replaces, val)
		case "backup":
			info.Backup = append(info.Backup, val)
		case "group":
			info.Groups = append(info.Groups, val)
		case "xdata":
			info.XData = append(info.XData, val)
		}
	}
	return info
}
