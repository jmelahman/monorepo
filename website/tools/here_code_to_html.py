import fileinput

from pygments import highlight
from pygments.lexers import get_lexer_by_name
from pygments.formatters import HtmlFormatter

def main():
    print(code_to_html(fileinput.input()))

def code_to_html(code, lexer = 'python3', linenos = True):
    formatter = HtmlFormatter(linenos=linenos,
                              noclasses=False,
                              cssclass='')
    html = highlight(code, get_lexer_by_name(lexer), formatter)
    return html

if __name__ == '__main__':
    main()
