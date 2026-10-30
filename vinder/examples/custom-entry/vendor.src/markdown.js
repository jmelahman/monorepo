// The entry names only what the page uses, so esbuild leaves out the rest of
// marked, and sets the global the page's plain <script> calls.
import { parse } from "marked";

window.renderMarkdown = (text) => parse(text);
