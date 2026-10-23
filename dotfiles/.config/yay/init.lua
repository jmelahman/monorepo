-- pkglint: lint each AUR PKGBUILD before yay shows menus or builds anything.
-- PKGLINT_FAIL_ON overrides the threshold per run (info|warn|error|critical).
local function shquote(s) return "'" .. s:gsub("'", [['\'']]) .. "'" end

yay.create_autocmd("AURPreInstall", {
  desc = "pkglint: refuse packages with static findings at or above the threshold",
  callback = function(event)
    local fail_on = os.getenv("PKGLINT_FAIL_ON") or "error"
    -- --no-inline-ignores: the maintainer's "# pkglint: ignore=" doesn't get to
    -- hide findings from the person installing the package.
    local status = os.execute("pkglint --no-inline-ignores --fail-on "
      .. shquote(fail_on) .. " " .. shquote(event.data.dir))
    if status == 0 then return end
    io.write(event.match .. ": pkglint reported findings. Continue anyway? [Y/n] ")
    io.flush()
    -- Enter continues; "n" or no terminal to read from (EOF) refuses.
    local answer = io.read("*l")
    if answer == nil or answer:match("^%s*[nN]") then
      yay.abort("refused by pkglint")
    end
  end,
})
