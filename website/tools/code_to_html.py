import sys

from os import listdir
from os.path import isfile, join
from pygments import highlight
from pygments.lexers import get_lexer_by_name
from pygments.formatters import HtmlFormatter

def main():
    chapter = ''
    exercise = ''
    try:
        chapter = str(sys.argv[1])
        exercise = str(sys.argv[2])
    except:
        pass
    my_path = '../../python-for-everybody-solutions/'
    only_files = [f for f in listdir(my_path) if isfile(join(my_path, f))]

    for filename in sorted(only_files):
        if not filename.endswith('.py'):
            continue
        if ((exercise and 'exercise' + chapter + '_' + exercise not in filename) or
                'exercise' + chapter not in filename):
            continue
        fhand = open(my_path + filename, "r")
        print(code_to_html(fhand.read()))

def code_to_html(code, lexer = 'python3', linenos = True):
    formatter = HtmlFormatter(linenos=linenos,
                              noclasses=False,
                              cssclass='')
    html = highlight(code, get_lexer_by_name(lexer), formatter)
    return html

if __name__ == '__main__':
    main()
