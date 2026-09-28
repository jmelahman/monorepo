// A profile that is nothing but 5 Wild in a window. It is launched by path
// (see 5-wild.desktop) rather than listed in profiles.ini, so the default
// profile stays the one plain `firefox` opens.

// Honor chrome/userChrome.css, which hides every toolbar.
user_pref("toolkit.legacyUserProfileCustomizations.stylesheets", true);

// With the tab strip hidden, any page Firefox opens on its own is an invisible
// tab stacked over the game, so switch off everything that opens one.
user_pref("browser.aboutwelcome.enabled", false);
user_pref("trailhead.firstrun.didSeeAboutWelcome", true);
user_pref("browser.startup.homepage_override.mstone", "ignore");
user_pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);
user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("browser.sessionstore.resume_from_crash", false);
user_pref("browser.tabs.warnOnClose", false);
