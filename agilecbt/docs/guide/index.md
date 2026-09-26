# Introduction

AgileCBT is a personal, self-hosted app for living with depression and anxiety.
It borrows the rhythm of agile teams and fills it with tools from cognitive
behavioral therapy (CBT):

| Agile | AgileCBT |
| --- | --- |
| Roadmap | Your **values** and the **goals** that serve them |
| Sprint | A **week**, with one intention |
| Board | **Steps** sized by energy: Someday → This week → Today → Done |
| Daily standup | A morning or evening **check-in** (mood, energy, anxiety), optionally with an AI coach |
| Retro | A weekly **retro**: what helped, what was hard, one thing to try |

On top of that are the CBT pieces:

- **Behavioral activation.** Rate mastery and pleasure when you finish a step.
- **Thought records.** A guided walk through a stressful thought, including
  common thinking traps.
- **A mood trend chart.**

It's gentle by design:

- Unfinished steps are *carried over* or *let go*, and nothing is ever
  "overdue".
- Every finished step counts.
- Today asks one question at a time, and the coach keeps your energy in mind
  when it suggests how much to take on.

::: info Not a replacement for care
AgileCBT is a self-help tool, and the AI coach is supportive, not a therapist.
If you're in crisis, call your local emergency number. In the US, call or
text 988. The coach shares a list of crisis lines if a conversation
calls for it, and you can [make it your own](/guide/configuration#crisis-resources).
:::

## What's in the box

A single `agilecbt` binary that contains:

- the web UI, installable on your phone's home screen
- a JSON REST API and CLI
- SQLite storage in your own data directory
- an [MCP server](/guide/ai) so Claude Code or Claude Desktop can help plan

Continue with [Install](/guide/install) or the [Quickstart](/guide/quickstart).
