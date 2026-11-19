package saga

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, s)
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

func (r *recorder) count(s string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.calls {
		if c == s {
			n++
		}
	}
	return n
}

func TestSagaAllSuccess(t *testing.T) {
	rec := &recorder{}
	mk := func(name string) Step {
		return Step{
			Name:       name,
			Do:         func(context.Context) error { rec.add("do:" + name); return nil },
			Compensate: func(context.Context) error { rec.add("undo:" + name); return nil },
		}
	}
	s := New([]Step{mk("order"), mk("stock"), mk("payment")}, Config{})
	if err := s.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := rec.snapshot()
	want := []string{"do:order", "do:stock", "do:payment"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestSagaCompensatesInReverse(t *testing.T) {
	rec := &recorder{}
	order := Step{Name: "order",
		Do:         func(context.Context) error { rec.add("do:order"); return nil },
		Compensate: func(context.Context) error { rec.add("undo:order"); return nil }}
	stock := Step{Name: "stock",
		Do:         func(context.Context) error { rec.add("do:stock"); return nil },
		Compensate: func(context.Context) error { rec.add("undo:stock"); return nil }}
	pay := Step{Name: "payment",
		Do:         func(context.Context) error { rec.add("do:payment"); return ErrPaymentDecline },
		Compensate: func(context.Context) error { rec.add("undo:payment"); return nil }}

	err := New([]Step{order, stock, pay}, Config{}).Run(context.Background())
	var se *SagaError
	if !errors.As(err, &se) {
		t.Fatalf("应返回 *SagaError，实际 %v", err)
	}
	if se.FailedStep != "payment" || !errors.Is(err, ErrPaymentDecline) {
		t.Fatalf("失败步骤/原因错误: %v", se)
	}
	want := []string{"do:order", "do:stock", "do:payment", "undo:stock", "undo:order"}
	got := rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("补偿顺序错误: got %v want %v", got, want)
		}
	}
	if se.Compensated[0] != "stock" {
		t.Fatalf("应先补偿最近完成的 stock，实际 %v", se.Compensated)
	}
}

func TestCompensationFailureDoesNotStopOthers(t *testing.T) {
	rec := &recorder{}
	a := Step{Name: "a",
		Do:         func(context.Context) error { return nil },
		Compensate: func(context.Context) error { rec.add("undo:a"); return nil }}
	b := Step{Name: "b",
		Do:         func(context.Context) error { return nil },
		Compensate: func(context.Context) error { rec.add("undo:b"); return errors.New("compensation failed") }}
	c := Step{Name: "c",
		Do:         func(context.Context) error { return ErrOutOfStock },
		Compensate: func(context.Context) error { return nil }}

	err := New([]Step{a, b, c}, Config{}).Run(context.Background())
	var se *SagaError
	if !errors.As(err, &se) {
		t.Fatalf("应返回 *SagaError，实际 %v", err)
	}
	if _, ok := se.CompensationFails["b"]; !ok {
		t.Fatalf("b 的补偿失败应被记录: %v", se.CompensationFails)
	}
	if rec.count("undo:a") != 1 {
		t.Fatalf("b 失败后仍应继续补偿 a，实际 %v", rec.snapshot())
	}
}

func TestCompensationRetries(t *testing.T) {
	var attempts int
	step := Step{Name: "s",
		Do: func(context.Context) error { return nil },
		Compensate: func(context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient")
			}
			return nil
		}}
	fail := Step{Name: "fail",
		Do:         func(context.Context) error { return errors.New("boom") },
		Compensate: func(context.Context) error { return nil }}

	err := New([]Step{step, fail}, Config{CompensateAttempts: 3, CompensateDelay: time.Millisecond}).Run(context.Background())
	if err == nil {
		t.Fatal("应失败")
	}
	if attempts != 3 {
		t.Fatalf("补偿应重试到第 3 次成功，实际 %d", attempts)
	}
}

func TestSagaCancelCompensates(t *testing.T) {
	rec := &recorder{}
	done := Step{Name: "done",
		Do:         func(context.Context) error { rec.add("do:done"); return nil },
		Compensate: func(context.Context) error { rec.add("undo:done"); return nil }}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := New([]Step{done}, Config{}).Run(ctx)
	if err == nil {
		t.Fatal("已取消的 ctx 应返回错误")
	}
	if rec.count("undo:done") != 0 {
		t.Fatal("未执行的步骤不应补偿")
	}

	// 执行到一半取消：已完成的步骤要补偿。
	ctx2, cancel2 := context.WithCancel(context.Background())
	slow := Step{Name: "slow",
		Do:         func(context.Context) error { rec.add("do:slow"); return nil },
		Compensate: func(context.Context) error { rec.add("undo:slow"); return nil }}
	block := Step{Name: "block",
		Do: func(c context.Context) error {
			cancel2()
			<-c.Done()
			return c.Err()
		},
		Compensate: func(context.Context) error { return nil }}
	if err := New([]Step{slow, block}, Config{}).Run(ctx2); err == nil {
		t.Fatal("应返回错误")
	}
	if rec.count("undo:slow") != 1 {
		t.Fatalf("应补偿已完成步骤 slow，实际 %v", rec.snapshot())
	}
}
