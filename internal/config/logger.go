package config

import (
	/*slog (Structured Logger) in Go 1.21+:
Standard library ka modern logger hai jo Key-Value pairs 
mein log karta hai eg time = ... level = ... msg="" types
 */
	"log/slog"
	"os"
)
// bhai isse apapna custom logger set kar sakte hai taaki apan ko debugging me asani hoan  
// Socho tumhara server production mein chal raha hai aur log aata hai:
/*Tum kya poochoge?
Kab hua? (Timestamp)
Kis client ke saath hua? (IP address)
Kaunse port pe hua? (Port)
Severity kya hai? (Warning hai ya Critical Crash)*/ 
// SetupLogger initializes the global structured logger with custom level and format.

func SetupLogger(debug bool) {
	// 1. slog ke 4 level hote - sequence  leveldebug,levelinfo,levelwarn,levelerror using them depends on usecase
	level := slog.LevelInfo // info level tk 
	if debug {
		level = slog.LevelDebug
	}

	// opts -> Yeh struct logger ko batata hai ki kitne level ke niche wale logs ignore karne hain.
	// yha pe level ke niche wale sare ignore krne h
	opts := &slog.HandlerOptions{
		Level: level,
	}
	// slog.NewTextHandler -> Text Handler human-readable format mein output deta hai (ideal for development/terminal).
	// slog.NewJSONHandler -> JSON format mein output deta hai (ideal for production/parsing).
	//handler Yeh decide karta hai ki log kis format mein aur kahan jayega.
	handler := slog.NewTextHandler(os.Stdout, opts)

	// 4. Default logger set karo taaki pura project slog.Info() use kar sake
	// Isse pura Go application ka global slog configured logger use karne lagta hai. 
	//Ab kisi bhi file mein tum seedha slog.Info(...) call karoge, wo is configuration ke according chalega
	logger := slog.New(handler)
	slog.SetDefault(logger)
}