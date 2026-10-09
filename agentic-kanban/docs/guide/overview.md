# Overview

The **overview** tab shows every board at once. A sidebar lists each board's columns and tickets, and clicking a ticket opens its session in a panel beside the list. Several panels can be open together, so you can watch sessions from different boards side by side.

Each ticket in the sidebar has a dot showing its session's status, updated live.

## Filtering the sidebar

Two buttons in the sidebar's **Boards** header narrow the list. Both are remembered per browser.

The first button cycles through three states:

| State   | Icon              | What the sidebar shows                                   |
| ------- | ----------------- | -------------------------------------------------------- |
| All     | List, muted       | Every board, column, and ticket.                         |
| Tickets | Fold, highlighted | Every ticket. Empty columns and empty boards are hidden. |
| Running | Pulse, highlighted | Only tickets with a running session.                    |

A session counts as running when it is starting, idle, working, or awaiting permission. Stopped and errored sessions don't count.

The second button, a window icon, shows only tickets that are open in a panel. It combines with the first: with **Running** selected too, a ticket must be both running and open. On a phone, where a ticket replaces the list instead of opening beside it, this button isn't shown.

While any filter is on, columns and boards with no matching tickets are hidden, so their **+** buttons are too. Switch back to **All** to add a ticket to an empty column.
