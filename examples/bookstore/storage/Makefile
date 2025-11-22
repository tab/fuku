.PHONY: run
run:
	sleep 3;
	@echo "INFO: Starting storage service..."
	@counter=0; \
	while true; do \
		case $$((counter % 10)) in \
			0) echo "level=info msg=\"File uploaded successfully\" size=2048" ;; \
			1) echo "level=debug msg=\"Cache invalidation triggered\"" ;; \
			2) echo "level=warn msg=\"Disk usage at 85%\"" ;; \
			3) echo "level=error msg=\"Failed to write to disk\"" ;; \
			4) echo "FATAL: Storage backend unreachable" ;; \
			5) echo "CRITICAL: Data corruption detected" ;; \
			6) echo "level=info msg=\"S3 object uploaded\" X-Trace-ID=5c6d7e8f-9a0b-1c2d-3e4f-5g6h7i8j9k0l Bucket=bookstore-prod Key=covers/978-0-123456-78-9.jpg Size=2457600 Content-Type=image/jpeg ETag=d41d8cd98f00b204e9800998ecf8427e Duration=1247ms Region=us-east-1" ;; \
			7) echo "level=error msg=\"Disk write failed\" X-Trace-ID=6d7e8f9a-0b1c-2d3e-4f5g-6h7i8j9k0l1m Path=/mnt/data/temp/upload_abc123.tmp Size=15728640 Available-Space=512MB Required-Space=16MB Error=ENOSPC Retry-Count=3 Operation=write" ;; \
			8) echo "level=warn msg=\"Storage quota warning\" X-Trace-ID=7e8f9a0b-1c2d-3e4f-5g6h-7i8j9k0l1m2n User-ID=user_corporate_001 Current-Usage=47.5GB Quota=50GB Percentage=95% Plan=enterprise Files-Count=12847 Last-Updated=$$(date)" ;; \
			9) echo "CRITICAL: Replication lag detected X-Trace-ID=8f9a0b1c-2d3e-4f5g-6h7i-8j9k0l1m2n3o Primary-Region=us-west-2 Replica-Region=eu-central-1 Lag=5432ms Last-Sync=$$(date -u +%Y-%m-%dT%H:%M:%SZ) Objects-Pending=1247 Status=degraded" ;; \
		esac; \
		counter=$$((counter + 1)); \
		sleep 10; \
	done
