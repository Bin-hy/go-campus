package producer_consumer

import (
	"testing"
	"time"
)

func TestRun_Basic(t *testing.T) {
	// 2个生产者，各产3个: [0,1,2] + [3,4,5] = 15
	result := Run(2, 3, 3)
	if result != 15 {
		t.Errorf("期望 15，得到 %d", result)
	}
}

func TestRun_SingleProducerSingleConsumer(t *testing.T) {
	// 1个生产者产5个: 0+1+2+3+4 = 10
	result := Run(1, 1, 5)
	if result != 10 {
		t.Errorf("期望 10，得到 %d", result)
	}
}

func TestRun_ManyProducers(t *testing.T) {
	// 10个生产者，各产10个: sum(0..99) = 4950
	result := Run(10, 5, 10)
	if result != 4950 {
		t.Errorf("期望 4950，得到 %d", result)
	}
}

func TestRun_ManyConsumers(t *testing.T) {
	// 3个生产者，各产4个: sum(0..11) = 66
	result := Run(3, 10, 4)
	if result != 66 {
		t.Errorf("期望 66，得到 %d", result)
	}
}

func TestRun_ZeroItems(t *testing.T) {
	result := Run(5, 5, 0)
	if result != 0 {
		t.Errorf("期望 0，得到 %d", result)
	}
}

func TestRun_Large(t *testing.T) {
	// 100个生产者，各产100个: sum(0..9999) = 49995000
	result := Run(100, 50, 100)
	expected := 100*100*(100*100-1) / 2 // n*(n-1)/2 where n=10000
	if result != expected {
		t.Errorf("期望 %d，得到 %d", expected, result)
	}
}

func TestRun_Terminates(t *testing.T) {
	done := make(chan struct{})
	go func() {
		Run(5, 5, 100)
		close(done)
	}()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("Run 未能在5秒内完成，可能存在死锁或 goroutine 泄漏")
	}
}
