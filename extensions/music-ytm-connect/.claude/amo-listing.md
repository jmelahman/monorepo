# AMO listing draft — Music Server Connect for YouTube Music

## Name
Music Server Connect for YouTube Music

## Summary (max 250 chars)
Connects your self-hosted music server to your YouTube Music account in a
click: sends your YouTube sign-in and your Supermix playlist straight to
the server you name, with no cookies.txt to export.

## Description
For people who run the open-source <a href="https://github.com/jmelahman/monorepo/tree/master/music">music</a>
server at home. The server streams YouTube Music mixes for your account,
which needs your YouTube sign-in cookies and your Supermix's playlist id.
This extension sends both from music.youtube.com in a click.

<b>What you get</b>
• Send my sign-in: YouTube's cookies from the window you click in, sent to your server
• Use this playlist as my Supermix: saves the open playlist on your server
• Test: asks your server to fetch the Supermix there and then

<b>Privacy</b>
• Your YouTube cookies are sent only to the server address you type in, and only when you click Send
• Firefox asks your permission for that one address before anything is sent
• No analytics, no other services; the address is kept in Firefox's synced storage
• Works in private windows and containers, sending that window's own sign-in

This extension is not made by or affiliated with YouTube or Google.

Source code: https://github.com/jmelahman/monorepo/tree/master/extensions/music-ytm-connect

## Categories
Other (primary); no secondary

## Tags
youtube music, self-hosted, homelab, music

## Homepage
https://github.com/jmelahman/monorepo/tree/master/extensions/music-ytm-connect

## Support email
lahmanja@gmail.com

## License
GNU General Public License v3.0

## Notes to reviewer
The extension reads youtube.com cookies (the `cookies` permission and the
youtube.com host permission) and, when the user clicks "Send my sign-in",
PUTs them in Netscape cookies.txt format to the user's own self-hosted
server, at an address the user types in and grants as an optional host
permission. That is the extension's whole purpose, declared as
`authenticationInfo` data collection. Nothing is sent anywhere else. To
try it without a server, the popup shows "Could not reach …" for any
address; the server is https://github.com/jmelahman/monorepo/tree/master/music
(`go run . serve`, listening on :8095).
