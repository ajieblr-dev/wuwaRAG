package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the application configuration parameters.
type Config struct {
	Port         string
	QdrantHost   string
	QdrantPort   int
	QdrantURL    string
	Collection   string
	VectorDim    int

	LiteLLMURL   string
	LiteLLMKey   string
	LiteLLMModel string

	GeminiAPIKey     string
	GroqAPIKey       string
	HuggingFaceKey   string
	EmbeddingModel   string
	EmbeddingBaseURL string

	WikiAPIURL       string
	DefaultWikiPage  string
	TopK             int
	ScoreThreshold   float64

	HybridSearchEnabled bool
	QueryRewriteEnabled bool
	RRFConstant         int
	CandidatePoolSize   int
}

// Load loads configuration from environment variables and an optional .env file.
func Load() *Config {
	// Load .env if present; ignore error if not found
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		QdrantHost:       getEnv("QDRANT_HOST", "localhost"),
		QdrantPort:       getEnvAsInt("QDRANT_PORT", 6333),
		QdrantURL:        getEnv("QDRANT_URL", "http://localhost:6333"),
		Collection:       getEnv("QDRANT_COLLECTION", "wuwa_lore"),
		VectorDim:        getEnvAsInt("VECTOR_DIM", 384),

		LiteLLMURL:       getEnv("LITELLM_URL", "http://localhost:4000"),
		LiteLLMKey:       getEnv("LITELLM_KEY", "sk-wuwa-rag-secret"),
		LiteLLMModel:     getEnv("LITELLM_MODEL", "wuwa-lore-model"),

		GeminiAPIKey:     getEnv("GEMINI_API_KEY", ""),
		GroqAPIKey:       getEnv("GROQ_API_KEY", ""),
		HuggingFaceKey:   getEnv("HUGGINGFACE_API_KEY", ""),
		EmbeddingModel:   getEnv("EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2"),
		EmbeddingBaseURL: getEnv("EMBEDDING_BASE_URL", "https://router.huggingface.co/hf-inference/models"),

		WikiAPIURL:       getEnv("WIKI_API_URL", "https://wutheringwaves.fandom.com/api.php"),
		DefaultWikiPage:  getEnv("DEFAULT_WIKI_PAGE", "Simulacrum_of_the_Heart"),
		TopK:             getEnvAsInt("TOP_K", 8),
		ScoreThreshold:   getEnvAsFloat("SCORE_THRESHOLD", 0.20),

		HybridSearchEnabled: getEnvAsBool("HYBRID_SEARCH_ENABLED", true),
		QueryRewriteEnabled: getEnvAsBool("QUERY_REWRITE_ENABLED", true),
		RRFConstant:         getEnvAsInt("RRF_K", 60),
		CandidatePoolSize:   getEnvAsInt("CANDIDATE_POOL_SIZE", 25),
	}

	return cfg
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	valStr := getEnv(key, "")
	if val, err := strconv.ParseFloat(valStr, 64); err == nil {
		return val
	}
	return defaultVal
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal
	}
	if b, err := strconv.ParseBool(valStr); err == nil {
		return b
	}
	return defaultVal
}
