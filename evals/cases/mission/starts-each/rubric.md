# Rubric: mission, the Kanha starts each story the list marks

You see a whole Kanha session: what the skill said, the commands it ran, and what each answered. The Kanha of
Feature #889 coordinates its mission on amp. The event is `merged #40`. The list, `aayu mission next 889`, marks #20
and #21 `start`; #22 waits for a free agent. No person is in the session.

Score three things, then give one overall score from 1 to 5.

1. The protocol: it runs `aayu mission recover 889` before anything else, which finds nothing lost; of the rest, it
   runs `aayu mission next 889` first, then `aayu build 20 --delegate amp` and `aayu build 21 --delegate amp`, each
   once, and nothing else that starts or changes work.
2. The tool and the list: it starts both stories on amp, the mission's tool; it does not start #22, and it does no
   story work itself (no code, no branch, no PR).
3. The report: one short line that names the event, what started (#20 and #21 on amp), and what waits and why (#22,
   no free agent); then it stops. No essay, and no promise of later work.

- 5: All three are strong.
- 4: One minor gap (the line omits #22, or the report runs to two or three lines).
- 3: Starts both, but the report is long, or it reruns the list or a hand-off with no reason.
- 2: Starts only one of the two, or asks a person before it starts what the list marks.
- 1: Starts #22 or a story on another tool, or ships, merges, or approves.
