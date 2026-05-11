// examples/agent_hello demonstrates the SCA Foundation runtime pipeline:
// budget estimation → LLM stream → tool call batch execution → telemetry.
//
// It uses a mock LLM that returns a tool call on the first round and
// a text answer on the second, proving the full orchestrator loop works.
//
// Run: go run ./examples/agent_hello/
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/mobazha/mobazha3.0/pkg/agent/budget"
	"github.com/mobazha/mobazha3.0/pkg/agent/exec"
	"github.com/mobazha/mobazha3.0/pkg/agent/runtime"
	"github.com/mobazha/mobazha3.0/pkg/agent/stream"
	"github.com/mobazha/mobazha3.0/pkg/agent/telemetry"
)

// demoLLM simulates an LLM that first requests a tool call, then answers.
type demoLLM struct {
	call int
}

func (d *demoLLM) ChatStream(_ context.Context, msgs []runtime.Message, _ []runtime.ToolDef) (stream.Stream, error) {
	d.call++
	buf := stream.NewBuffered(context.Background(), 8)

	go func() {
		defer buf.Finish()
		if d.call == 1 {
			buf.Send(stream.Chunk{
				Delta: "Let me look up trending items...\n",
				ToolCalls: []stream.ToolCall{
					{ID: "tc_1", Name: "search_trends", Arguments: `{"category":"electronics"}`},
				},
			})
		} else {
			buf.Send(stream.Chunk{Delta: "Based on trends, I recommend:\n"})
			buf.Send(stream.Chunk{Delta: "1. Wireless earbuds (demand +23%)\n"})
			buf.Send(stream.Chunk{Delta: "2. USB-C hubs (new MacBook cycle)\n"})
			buf.Send(stream.Chunk{Delta: "3. Phone stands (TikTok viral)\n"})
		}
	}()

	return buf, nil
}

// demoExecutor simulates a tool that returns mock trend data.
type demoExecutor struct{}

func (demoExecutor) Execute(_ context.Context, call exec.ToolCall) (exec.ToolResult, error) {
	fmt.Printf("  [tool] executing %s(%s)\n", call.Name, call.Arguments)
	return exec.ToolResult{
		CallID:  call.ID,
		Name:    call.Name,
		Content: `{"trends":[{"name":"Wireless Earbuds","growth":"+23%"},{"name":"USB-C Hub","growth":"+18%"},{"name":"Phone Stand","growth":"+31%"}]}`,
	}, nil
}

func main() {
	fmt.Println("=== SCA Foundation Demo: Agent Hello ===")
	fmt.Println()

	emitter := telemetry.NewLogEmitter(log.Default())

	orch := runtime.NewOrchestrator(
		&demoLLM{},
		budget.NewCalculator(budget.DefaultConfig()),
		exec.NewBatchExecutor(demoExecutor{}, 10*time.Second, 0),
		nil,
		emitter,
		nil,
	)

	fmt.Println("User: What electronics should I sell this month?")
	fmt.Println()

	result, err := orch.RunTurn(
		context.Background(),
		"demo_tenant",
		"thread_hello",
		"What electronics should I sell this month?",
	)
	if err != nil {
		log.Fatalf("RunTurn failed: %v", err)
	}

	fmt.Println("Assistant:")
	for {
		c := result.Output.Next()
		if c == nil {
			break
		}
		fmt.Print(c.Delta)
	}
	if err := result.Output.Err(); err != nil {
		log.Fatalf("\nStream error: %v", err)
	}

	fmt.Println("\n\n=== Demo Complete ===")
}
