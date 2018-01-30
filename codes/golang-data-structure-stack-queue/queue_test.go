package queue

import "testing"

var queue ItemQueue

func initQueue() *ItemQueue {
	if queue.items == nil {
		queue = ItemQueue{}
		queue.New()
		return &queue
	}
	return &queue
}

func TestEnqueue(t *testing.T) {
	queue := initQueue()
	queue.Enqueue(1)
	queue.Enqueue(2)
	queue.Enqueue(3)
	if size := queue.Size(); size != 3 {
		t.Errorf("Wrong count, expected 3 and got %d", size)
	}
}

func TestDequeue(t *testing.T) {
	queue.Dequeue()
	if size := len(queue.items); size != 2 {
		t.Errorf("Wrong count, expected 2 and got %d", size)
	}

	queue.Dequeue()
	queue.Dequeue()
	if size := len(queue.items); size != 0 {
		t.Errorf("Wrong count, expected 0 and got %d", size)
	}

	if !queue.IsEmpty() {
		t.Errorf("IsEmpty should return true")
	}
}
