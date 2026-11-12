Output out/raw.mp4
Set FontSize 32
Source head.tape

Type "curl -s localhost:8080/health && echo"
Sleep 400ms
Enter
Sleep 2s

Type "fuku run"
Sleep 400ms
Enter
Wait+Screen@60s /running 6\/6 ready/
Sleep 4s
