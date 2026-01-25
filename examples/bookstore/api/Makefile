.PHONY: run
run:
	@echo "INFO: [API] Starting api service (PID: $$$$)..."
	@sleep 2
	@echo "INFO: [API] Service ready, accepting requests..."
	@cleanup() { \
		echo "INFO: [API] Received shutdown signal, starting graceful shutdown..."; \
		echo "INFO: [API] Draining active connections..."; \
		sleep 1; \
		echo "INFO: [API] Closing database connections..."; \
		sleep 1; \
		echo "INFO: [API] Flushing caches..."; \
		sleep 1; \
		echo "INFO: [API] Shutdown complete, goodbye!"; \
		exit 0; \
	}; \
	trap cleanup SIGTERM SIGINT; \
	counter=0; \
	while true; do \
		case $$((counter % 8)) in \
			0) echo "INFO: [API] Processing request at $$(date)" ;; \
			1) echo "DEBUG: [API] Cache hit for user data" ;; \
			2) echo "WARN: [API] Slow query detected (245ms)" ;; \
			3) echo "ERROR: [API] Failed to connect to database" ;; \
			4) echo "INFO: Request GET /api/v1/books processed successfully X-Trace-ID=7a8b9c0d-1e2f-3a4b-5c6d-7e8f9a0b1c2d Session-ID=sess_abc123xyz456 User-Agent=Mozilla/5.0 Response-Time=127ms Status=200" ;; \
			5) echo "ERROR: Database connection timeout X-Trace-ID=8b9c0d1e-2f3a-4b5c-6d7e-8f9a0b1c2d3e Session-ID=sess_def456uvw789 Request-ID=req_12345 Endpoint=/api/v1/orders User-ID=user_98765 Duration=5023ms Error=connection_timeout" ;; \
			6) echo "WARN: Rate limit approaching threshold X-Trace-ID=9c0d1e2f-3a4b-5c6d-7e8f-9a0b1c2d3e4f Client-IP=192.168.1.105 API-Key=ak_prod_abc123 Current-Rate=95/100 Window=60s Endpoint=/api/v1/search" ;; \
			7) echo "DEBUG: Cache invalidation triggered for key=books:category:fiction X-Trace-ID=0d1e2f3a-4b5c-6d7e-8f9a-0b1c2d3e4f5g TTL=3600s Reason=manual_flush Timestamp=$$(date +%s)" ;; \
		esac; \
		counter=$$((counter + 1)); \
		sleep 2 & wait; \
	done
