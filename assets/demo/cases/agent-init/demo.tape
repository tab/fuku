Output out/raw.mp4
Set FontSize 16
Source head.tape

Ctrl+B
Right
Sleep 300ms
Type "Write a fuku.yaml for the services in examples/bookstore"
Sleep 500ms
Enter
Wait+Screen@240s /done \d/
Sleep 4s

Ctrl+B
Left
Sleep 300ms
Type "fuku run"
Sleep 400ms
Enter
Wait+Screen@60s /running \d\/\d ready/
Sleep 6s
