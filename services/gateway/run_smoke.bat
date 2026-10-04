@echo off
set DATABASE_URL=postgresql://neondb_owner:npg_lrE4mfqUi5Nn@ep-falling-math-b37vc0lc-pooler.c-4.ap-southeast-1.aws.neon.tech/neondb?channel_binding=require&sslmode=require
set PANTA_API_URL=https://live-api.panta.market/api/v1/
set PANTA_API_KEY=pk_live_pk_live_LSxLji63bHRj5H81Mr6nkUsu1Jr3wydzUpR4IiwLZ6Q
set SOLANA_RPC_URL=https://mainnet.helius-rpc.com/?api-key=4db9eb53-c22a-4670-9f2a-b1ca61563216
set PANTA_SMOKE_TEST_ENABLED=true
set JWT_SECRET=test-secret-test-secret-test-secret-test-secret
set PORT=8080
go run .