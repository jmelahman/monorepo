#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.11"
# dependencies = [
#   "markdown-it-py>=3,<5",
#   "pygments>=2.17,<3",
# ]
# ///
"""Render Markdown pages into template.html.

Each FILE is written next to itself as .html. With no FILE, Markdown is read
from stdin and written to --output.

The template uses the subset of pandoc's template syntax that template.html
needs ($var$, $if()$/$elseif()$/$else$/$endif$ and $for()$/$endfor$), so the
same file still works with the pandoc invocation in resume/build.
"""

from __future__ import annotations

import argparse
import html
from pathlib import Path
import re
import sys
from typing import TYPE_CHECKING

from markdown_it import MarkdownIt
from pygments import lex
from pygments.lexers import get_lexer_by_name
from pygments.token import _TokenType
from pygments.token import Token
from pygments.util import ClassNotFound

if TYPE_CHECKING:
    from collections.abc import Iterator

ROOT = Path(__file__).resolve().parent.parent
TEMPLATE = ROOT / "template.html"

# Pygments token types mapped onto the class names css/kate.css styles.
TOKEN_CLASSES = {
    Token.Comment: "co",
    Token.Comment.Preproc: "pp",
    Token.Keyword: "kw",
    Token.Keyword.Constant: "cn",
    Token.Keyword.Namespace: "im",
    Token.Keyword.Type: "dt",
    Token.Literal.Date: "dv",
    Token.Name.Attribute: "dt",
    Token.Name.Builtin: "bu",
    Token.Name.Class: "fu",
    Token.Name.Constant: "cn",
    Token.Name.Decorator: "at",
    Token.Name.Function: "fu",
    Token.Name.Tag: "fu",
    Token.Name.Variable: "va",
    Token.Number: "dv",
    Token.Number.Float: "fl",
    Token.Number.Hex: "bn",
    Token.Operator: "op",
    Token.Operator.Word: "kw",
    Token.Punctuation: "op",
    Token.String: "st",
    Token.String.Doc: "do",
    Token.String.Escape: "sc",
}
# Rendered as plain <pre class="lang"> blocks, which the stylesheet styles.
PLAIN_LANGS = {"shell"}

DIRECTIVE = re.compile(r"\$(if|elseif|for)\(([\w.-]+)\)\$|\$(else|endif|endfor)\$|\$([\w.-]+)\$")
# A control directive alone on a line does not leave a blank line behind.
STANDALONE = re.compile(
    r"^[ \t]*(\$(?:if|elseif|for)\([\w.-]+\)\$|\$(?:else|endif|endfor)\$)[ \t]*\n", re.MULTILINE
)


def token_class(ttype: _TokenType | None) -> str | None:
    while ttype is not None:
        if ttype in TOKEN_CLASSES:
            return TOKEN_CLASSES[ttype]
        ttype = ttype.parent
    return None


def render_fence(self, tokens, idx, options, env) -> str:  # noqa: ANN001, ARG001
    token = tokens[idx]
    lang = token.info.strip().split(maxsplit=1)[0] if token.info.strip() else ""
    code = token.content
    if not lang:
        return f"<pre><code>{html.escape(code)}</code></pre>\n"
    try:
        lexer = None if lang in PLAIN_LANGS else get_lexer_by_name(lang)
    except ClassNotFound:
        lexer = None
    if lexer is None:
        return f'<pre class="{html.escape(lang)}"><code>{html.escape(code)}</code></pre>\n'

    # One span per line, as css/kate.css expects.
    lines: list[list[tuple[str | None, str]]] = [[]]
    for ttype, value in lex(code.rstrip("\n"), lexer):
        cls = token_class(ttype)
        for i, part in enumerate(value.split("\n")):
            if i:
                lines.append([])
            if not part:
                continue
            line = lines[-1]
            if line and line[-1][0] == cls:
                line[-1] = (cls, line[-1][1] + part)
            else:
                line.append((cls, part))
    if not lines[-1]:
        lines.pop()
    body = "\n".join(
        "<span>"
        + "".join(
            f'<span class="{cls}">{html.escape(text)}</span>' if cls else html.escape(text)
            for cls, text in line
        )
        + "</span>"
        for line in lines
    )
    name = html.escape(lang)
    return (
        f'<div class="sourceCode"><pre class="sourceCode {name}">'
        f'<code class="sourceCode {name}">{body}</code></pre></div>\n'
    )


def slugify(text: str, seen: dict[str, int]) -> str:
    slug = re.sub(r"[^\w\s.-]", "", text.lower())
    slug = re.sub(r"\s+", "-", slug.strip())
    slug = re.sub(r"^[^a-z]+", "", slug) or "section"
    count = seen.get(slug, 0)
    seen[slug] = count + 1
    return f"{slug}-{count}" if count else slug


def make_parser() -> MarkdownIt:
    md = MarkdownIt("commonmark", {"html": True, "typographer": True})
    md.enable(["table", "strikethrough", "replacements", "smartquotes"])
    md.add_render_rule("fence", render_fence)
    return md


def render_markdown(md: MarkdownIt, source: str) -> str:
    tokens = md.parse(source)
    seen: dict[str, int] = {}
    for i, token in enumerate(tokens):
        if token.type == "heading_open":
            text = "".join(
                child.content
                for child in tokens[i + 1].children or []
                if child.type in ("text", "code_inline")
            )
            token.attrSet("id", slugify(text, seen))
    return md.renderer.render(tokens, md.options, {})


def split_front_matter(text: str) -> tuple[dict[str, str], str]:
    """Split off a leading `key: value` front matter block."""
    if not text.startswith("---\n"):
        return {}, text
    head, sep, body = text[4:].partition("\n---\n")
    if not sep:
        return {}, text
    meta: dict[str, str] = {}
    for line in head.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, colon, value = line.partition(":")
        if not colon:
            raise ValueError(f"unsupported front matter line: {line!r}")
        value = value.strip()
        if len(value) > 1 and value[0] == value[-1] and value[0] in "\"'":
            value = value[1:-1]
        meta[key.strip()] = value
    return meta, body


def truthy(value: object) -> bool:
    return bool(value) and value != "false"


def tokenize_template(text: str) -> Iterator[tuple[str, str]]:
    # DIRECTIVE has four groups, so the split alternates one literal with four group values.
    parts = DIRECTIVE.split(STANDALONE.sub(r"\1", text))
    for i in range(0, len(parts), 5):
        if parts[i]:
            yield "text", parts[i]
        if i + 1 < len(parts):
            opener, arg, closer, var = parts[i + 1 : i + 5]
            yield ("var", var) if var else (opener or closer, arg)


def parse_block(
    tokens: Iterator[tuple[str, str]], terminators: tuple[str, ...] = ()
) -> tuple[list[tuple], str | None, str | None]:
    """Parse nodes up to one of `terminators`, returning it along with its argument."""
    nodes: list[tuple] = []
    for kind, arg in tokens:
        if kind in ("text", "var"):
            nodes.append((kind, arg))
        elif kind == "if":
            nodes.append(("if", parse_branches(tokens, arg)))
        elif kind == "for":
            nodes.append(("for", arg, parse_block(tokens, ("endfor",))[0]))
        elif kind in terminators:
            return nodes, kind, arg
        else:
            raise ValueError(f"unexpected ${kind}$ in template")
    if terminators:
        raise ValueError(f"template is missing ${terminators[-1]}$")
    return nodes, None, None


def parse_branches(tokens: Iterator[tuple[str, str]], cond: str | None) -> list[tuple]:
    branches = []
    while True:
        children, end, arg = parse_block(tokens, ("elseif", "else", "endif"))
        branches.append((cond, children))
        if end == "endif":
            return branches
        cond = arg if end == "elseif" else None


def render_template(nodes: list[tuple], variables: dict[str, object]) -> str:
    out: list[str] = []
    for node in nodes:
        if node[0] == "text":
            out.append(node[1])
        elif node[0] == "var":
            out.append(str(variables.get(node[1], "")))
        elif node[0] == "if":
            for cond, children in node[1]:
                if cond is None or truthy(variables.get(cond)):
                    out.append(render_template(children, variables))
                    break
        else:
            _, name, children = node
            value = variables.get(name)
            items = value if isinstance(value, list) else [value] if truthy(value) else []
            out.extend(render_template(children, {**variables, name: item}) for item in items)
    return "".join(out)


def render_page(
    md: MarkdownIt, template: list[tuple], source: str, variables: dict[str, str]
) -> str:
    meta, body = split_front_matter(source)
    # Command-line variables win over front matter, as with pandoc.
    page: dict[str, object] = {
        k: html.escape(v, quote=True) for k, v in {**meta, **variables}.items()
    }
    if "date" in page:
        page.setdefault("date-meta", page["date"])
    page["body"] = render_markdown(md, body).rstrip("\n")
    return render_template(template, page)


def main() -> int:
    parser = argparse.ArgumentParser(description="Render Markdown pages into template.html.")
    parser.add_argument("files", nargs="*", type=Path, metavar="FILE")
    parser.add_argument("-o", "--output", type=Path, help="output path when reading stdin")
    parser.add_argument(
        "-V",
        "--variable",
        action="append",
        default=[],
        metavar="KEY=VALUE",
        help="set a template variable",
    )
    args = parser.parse_args()
    if bool(args.files) == bool(args.output):
        parser.error("pass either FILEs, or --output to render stdin")

    variables: dict[str, str] = {}
    for item in args.variable:
        key, _, value = item.partition("=")
        variables[key] = value or "true"
    md = make_parser()
    template = parse_block(tokenize_template(TEMPLATE.read_text()))[0]
    if args.output:
        args.output.write_text(render_page(md, template, sys.stdin.read(), variables))
    for path in args.files:
        path.with_suffix(".html").write_text(render_page(md, template, path.read_text(), variables))
    return 0


if __name__ == "__main__":
    sys.exit(main())
