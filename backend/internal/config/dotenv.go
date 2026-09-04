package config

import "github.com/joho/godotenv"

func loadDotEnv() error {
	// Prefer CWD .env (baked at /app/.env in Vercel image), then env.vercel next to the binary.
	_ = godotenv.Load(".env")
	_ = godotenv.Load("env.vercel")
	return nil
}
