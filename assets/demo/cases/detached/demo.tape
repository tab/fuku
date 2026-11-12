Output out/raw.mp4
Set FontSize 32
Source head.tape

Type "fuku run core -d"
Sleep 400ms
Enter
Wait+Screen@60s /Running detached/
Sleep 2s

Type "curl -s localhost:8080/health && echo"
Sleep 400ms
Enter
Sleep 1.5s

Type "fuku logs api --tail 5 --no-follow"
Sleep 400ms
Enter
Sleep 3s

Type "fuku stop"
Sleep 400ms
Enter
Wait+Screen@30s /stopped in/
Sleep 3s
