.PHONY: run
run:
	@echo "INFO: [AUTH] Starting auth service (PID: $$$$)..."
	@sleep 1
	@echo "INFO: [AUTH] Loading JWT keys..."
	@sleep 1
	@echo "INFO: [AUTH] Service ready, accepting requests..."
	@cleanup() { \
		echo "INFO: [AUTH] Received shutdown signal, starting graceful shutdown..."; \
		echo "INFO: [AUTH] Invalidating active sessions..."; \
		sleep 1; \
		echo "INFO: [AUTH] Revoking temporary tokens..."; \
		sleep 1; \
		echo "INFO: [AUTH] Shutdown complete, goodbye!"; \
		exit 0; \
	}; \
	trap cleanup SIGTERM SIGINT; \
	counter=0; \
	while true; do \
		case $$((counter % 9)) in \
			0) echo "[INFO] User authentication successful" ;; \
			1) echo "[DEBUG] Token validation passed" ;; \
			2) echo "[WARN] Password expires in 3 days" ;; \
			3) echo "[ERROR] Invalid credentials provided" ;; \
			4) echo "level=info msg=\"Session created for user:john\"" ;; \
			5) echo "[INFO] Authentication successful for user X-Trace-ID=1e2f3a4b-5c6d-7e8f-9a0b-1c2d3e4f5g6h Session-ID=sess_auth_xyz789 User-ID=user_john_doe Email=john.doe@example.com IP=10.0.1.42 User-Agent=curl/7.88.1 Method=POST Endpoint=/auth/login" ;; \
			6) echo "[ERROR] Failed authentication attempt X-Trace-ID=2f3a4b5c-6d7e-8f9a-0b1c-2d3e4f5g6h7i Session-ID=sess_fail_abc123 Username=invalid_user IP=203.0.113.45 Reason=invalid_password Attempts=3 Lockout=true Duration=245ms" ;; \
			7) echo "[WARN] Token refresh required X-Trace-ID=3a4b5c6d-7e8f-9a0b-1c2d-3e4f5g6h7i8j User-ID=user_jane_smith Token-Age=55m Expiry=5m Endpoint=/api/protected Scopes=read:profile,write:settings Session-ID=sess_refresh_def456" ;; \
			8) echo "level=debug msg=\"OAuth callback received\" X-Trace-ID=4b5c6d7e-8f9a-0b1c-2d3e-4f5g6h7i8j9k Provider=google State=state_abc123 Code=code_xyz789 Redirect-URI=https://bookstore.example.com/auth/callback Session-ID=sess_oauth_ghi789" ;; \
		esac; \
		counter=$$((counter + 1)); \
		sleep 3 & wait; \
	done
