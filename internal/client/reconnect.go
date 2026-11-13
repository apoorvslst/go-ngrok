/*Abhi tak agar WiFi 2 second ke liye drop ho jaye ya server restart ho jaye, toh hamara client turant crash/exit ho jata hai.
Client Auto-Reconnect Engine: AWS standard ka Exponential Backoff + Full Jitter jo connection tootne par intelligently retry karega bina server ko crash kiye (Thundering Herd Prevention).*/

/*concept:= 1. Pehle Concept Samjho: "Exponential Backoff + Full Jitter"
Thundering Herd Problem kya hai?
Agar 1,000 developers ngrok use kar rahe hain aur server 2 second ke liye reboot hua, aur sabhi clients time.Sleep(1 * time.Second) par retry karenge, toh 1,000 clients ek hi millisecond mein server par attack karenge aur server aate hi dobara crash ho jayega!
The Mathematical Solution:
Exponential Backoff: Wait time double karo:
Wait=BaseDelay×2^(attempt−1)
Attempt 1 = 1s, Attempt 2 = 2s, Attempt 3 = 4s, Attempt 4 = 8s... (Max 30s tak).
2) Full Jitter (AWS Architecture Pattern): Sabka timing randomize karo taaki 1,000 clients ka retry timeline par smoothly spread ho jaye:
jitterdelay =random(0,CalculatedWait)
*/

package client
import (
	"context"
	"log/slog"
	"math"
	"math/rand"
	 "time"
)
// BackoffConfig controls retry parameters.
type BackoffConfig struct{
	BaseDelay time.Duration   // 1*time.Second
	MaxDelay time.Duration    // 30*time.Second
	MaxRetries int              // 0 means unlimited tries
}
func RetryWithBackoff(ctx context.Context,cfg BackoffConfig,opName string,op func() error) error{
	attempt:=0
	for{
		attempt++
		err:=op()
		if err== nil{
			return nil
		}
// Agar user ne Ctrl+C daba diya hai toh retry loop band karo
		if ctx.Err()!=nil{
			return ctx.Err()
		}
		if cfg.MaxRetries>0 && attempt>=cfg.MaxRetries{
			return err
		}
		// 1. Calculate Exponential Backoff: Base * 2^(attempt-1)
		multiplier := math.Pow(2, float64(attempt-1))
		calculated := float64(cfg.BaseDelay) * multiplier

		// 2. Cap at MaxDelay taaki infinite sleep na ho
		if calculated > float64(cfg.MaxDelay) {
			calculated = float64(cfg.MaxDelay)
		}

		// 3. Add Full Jitter: random duration between 0 and calculated
		jittered := time.Duration(rand.Float64() * calculated)
		//AWS Architecture blog recommended "Full Jitter" implementation.
		slog.Warn("Operation failed,backing off before retry","operation", opName,
			"attempt", attempt,
			"retry_in", jittered.Round(time.Millisecond).String(),
			"err", err,)
		
		// 4. Sleep with context awareness (Ctrl+C dabate hi instant exit!)
		select {
		case <-ctx.Done():
			// ctx.Done(): Ek channel hota hai. Jab tak sab theek hai, yeh chup rehta hai. Jaise hi koi cancel karta hai ya timeout hota hai, yeh channel turant signal bhejta hai saves memory leak
			return ctx.Err()
		case <-time.After(jittered):   
		//Agar retry ka wait 15 second ka hai aur user terminal mein Ctrl+C dabata hai, toh normal time.Sleep program ko 15 second tak freeze rakhta hai.
		//  Lekin select with ctx.Done() turant cancel signal pakad leta hai aur bina wait kiye instant cleanly exit karta hai!
		}
	}
}