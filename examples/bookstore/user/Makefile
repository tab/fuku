.PHONY: run
run:
	@echo "[INFO] Starting user service..."
	@counter=0; \
	while true; do \
		case $$((counter % 7)) in \
			0) echo "info: User profile updated" ;; \
			1) echo "debug: Fetching user preferences" ;; \
			2) echo "warning: Rate limit approaching" ;; \
			3) echo "error: User not found in database" ;; \
			4) echo "info: User profile updated successfully X-Trace-ID=9a0b1c2d-3e4f-5g6h-7i8j-9k0l1m2n3o4p User-ID=user_alice_wonder Email=alice@wonderland.com Fields-Updated=display_name,avatar_url,bio,timezone Previous-Values={display_name:Alice,timezone:UTC} New-Values={display_name:Alice Wonder,timezone:America/New_York} Session-ID=sess_update_jkl012" ;; \
			5) echo "error: User lookup failed X-Trace-ID=0b1c2d3e-4f5g-6h7i-8j9k-0l1m2n3o4p5q Query-Type=email Query-Value=nonexistent@example.com Database=users_master Execution-Time=342ms Error-Code=USER_NOT_FOUND Retry-Attempted=false Caller-Service=api-gateway" ;; \
			6) echo "warning: Concurrent profile update detected X-Trace-ID=1c2d3e4f-5g6h-7i8j-9k0l-1m2n3o4p5q6r User-ID=user_bob_builder Session-IDs=sess_web_mno345,sess_mobile_pqr678 Conflict-Field=preferences.notifications Last-Modified-Web=$$(date -u +%Y-%m-%dT%H:%M:%SZ) Last-Modified-Mobile=$$(date -u +%Y-%m-%dT%H:%M:%SZ) Resolution=last_write_wins" ;; \
		esac; \
		counter=$$((counter + 1)); \
		sleep 15; \
	done
