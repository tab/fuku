.PHONY: run
run:
	@echo "INFO: Starting frontend-api service..."
	@counter=0; \
	while true; do \
		case $$((counter % 9)) in \
			0) echo "[INFO] Request received from client" ;; \
			1) echo "[DEBUG] Response time: 45ms" ;; \
			2) echo "[WARN] Client using deprecated API endpoint" ;; \
			3) echo "[ERROR] Downstream service unavailable" ;; \
			4) echo "ERR: Timeout waiting for backend" ;; \
			5) echo "[INFO] GraphQL query executed X-Trace-ID=2d3e4f5g-6h7i-8j9k-0l1m-2n3o4p5q6r7s Session-ID=sess_graphql_stu901 Query-Name=GetBookDetails Variables={bookId:978-0-123456-78-9} Complexity-Score=42 Execution-Time=156ms Cache-Hit=false Resolver-Calls=7 Database-Queries=3" ;; \
			6) echo "[ERROR] Circuit breaker opened for downstream service X-Trace-ID=3e4f5g6h-7i8j-9k0l-1m2n-3o4p5q6r7s8t Service=inventory-api Endpoint=https://inventory.internal:8080/api/v1/stock Failure-Rate=67% Threshold=50% Window=60s Failed-Requests=20/30 State=open Reset-Timeout=30s" ;; \
			7) echo "[WARN] API version deprecation notice X-Trace-ID=4f5g6h7i-8j9k-0l1m-2n3o-4p5q6r7s8t9u Client-Version=v1.2.3 Deprecated-Endpoint=/api/v1/cart Current-Version=v2.0.0 Migration-Guide=https://docs.bookstore.com/migration Sunset-Date=2024-12-31 User-Agent=BookstoreApp/1.2.3 (iOS 16.0)" ;; \
			8) echo "[DEBUG] Request pipeline trace X-Trace-ID=5g6h7i8j-9k0l-1m2n-3o4p-5q6r7s8t9u0v Middleware-Chain=cors,auth,rate-limit,logging,compression Auth-Time=12ms Rate-Limit-Time=3ms Compression-Ratio=3.2x Original-Size=8192 Compressed-Size=2560 Total-Overhead=18ms Request-Size=245 Response-Size=2560" ;; \
		esac; \
		counter=$$((counter + 1)); \
		sleep 3; \
	done
