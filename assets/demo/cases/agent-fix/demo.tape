Output out/raw.mp4
Set FontSize 22
Source head.tape

Type "fuku run"
Sleep 400ms
Enter
Wait+Screen@60s /max retries exceeded/
Sleep 1.5s

Ctrl+B
Right
Sleep 300ms
Type "api is down. Find out why, fix it and bring it back up"
Sleep 500ms
Enter
Wait+Screen@120s /done \d/
Sleep 8s
