import operator

from os import listdir
from os.path import isfile, join

files_dict = {}

my_path = "./images/gallery"
only_files = [f for f in listdir(my_path) if isfile(join(my_path, f))]

# The beginning float of the file names roughly translates to when the file was
# created with large numbers being created after smaller.
for file in only_files:
    temp_file = file
    files_dict[file] = float(temp_file.split("_")[0])

sorted_files = sorted(files_dict.items(), key=operator.itemgetter(1), reverse=True)

# Go through each file name and create the html code.
i = len(sorted_files)
for key, file in enumerate(sorted_files):
    html = '        <div id="' + str(i) + '" class="frame">\n' \
         + '          <div class="primary-border border-radius-16px">\n' \
         + '            <a href="/gallery#' + str(i) + '">\n' \
         + '              <img src="/images/gallery/' + file[0] + '" alt="TODO(jamison) Add alt"/>\n' \
         + '            </a>\n' \
         + '            <p class="caption"></p>\n' \
         + '          </div>\n' \
         + '        </div>'
    i -= 1
    print(html)
