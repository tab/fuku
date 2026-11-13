Output out/raw.mp4
Set FontSize 22
Source head.tape

Type "fuku run"
Sleep 400ms
Enter
Wait+Screen@60s /process exited/
Sleep 1.5s

Ctrl+B
Right
Sleep 300ms
Type "api keeps crashing, find the bug and fix it"
Sleep 500ms
Enter
Wait+Screen@180s /done \d/
Sleep 8s
