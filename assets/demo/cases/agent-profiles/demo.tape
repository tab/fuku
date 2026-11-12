Output out/raw.mp4
Set FontSize 16
Source head.tape

Type "Start the core profile"
Sleep 500ms
Enter
Wait+Screen@120s /done \d/
Sleep 4s

Type "Switch to minimal and show me what is running"
Sleep 500ms
Enter
Wait+Screen@120s /done \d[\s\S]*done \d/
Sleep 6s
