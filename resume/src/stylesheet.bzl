_BASE_STYLESHEET = "//resume/src:base.css"

def themed_stylesheet(name, src, **kwargs):
  """Concatenates a base stylesheet with one containing theming."""
  native.genrule(
    name = name,
    srcs = [src, _BASE_STYLESHEET],
    outs = ["{name}.css".format(name = name)],
    cmd = "echo '<style type=\"text/css\">' > $@; " +
          "cat $(location {base}) $(location {theme}) >> $@; ".format(base = _BASE_STYLESHEET, theme = src) +
          "echo '</style>' >> $@",
    **kwargs
  )
