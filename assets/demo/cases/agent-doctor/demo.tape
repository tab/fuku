Output out/raw.mp4
Set FontSize 16
Source head.tape

Type "fuku run"
Sleep 400ms
Enter
Sleep 2.5s

Ctrl+B
Right
Sleep 300ms
Type "fuku won't start. Find out why and fix it"
Sleep 500ms
Enter
Wait+Screen@180s /done \d/
Sleep 3s

Ctrl+B
Left
Sleep 300ms
Type "fuku doctor --summary"
Sleep 400ms
Enter
Sleep 6s
