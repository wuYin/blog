---
title: 进程间通信：管道
date: 2019-02-19 20:17:19
tags: 操作系统
---

Pipe IPC 在 Go 中的使用与实现。

<!-- more -->

## 管道（Pipe）

多个进程在**协作**完成同一任务时，通常彼此要**传输数据**，**共享资源**。
在 shell 中常常会用到管道符，如查看占用 80 端口的进程：`netstat -an | grep :80`，在 bash 中每个命令在执行时都是独立的进程，`netstat` 父进程通过管道将数据传输给 **fork** 出的 `grep` 子进程处理。这就是最简单的 IPC 管道通信。

### 分类

- 匿名管道：shell 中的 pipe 就是匿名管道，只能在父子进程 / 有亲缘关系的进程之间使用。
  原因：管道在 Linux 中是文件，想要通过匿名管道来读写数据，必须拥有相同的文件描述符，而拥有相同 fd 的两个进程需有亲缘关系。
- 命名管道：允许无亲缘关系的进程间传输数据。

### 特点

- 半双工：数据只能是单向流动的（优点：简单，缺点：单向）
- 面向字节流：管道中的数据是原生的字节流（优点：职责单一，缺点：相比消息队列实现的 IPC，无法选择接收或丢弃发来的数据）



## 匿名管道

 `os/exec` 包支持在执行的系统命令上建立匿名管道：

```go
func stdoutPipe() {
	echo := exec.Command("echo", "-n", "the quick brown fox jumps over the lazy dog")
	pipe, err := echo.StdoutPipe() // 获取命令执行后的 pipe
	if err != nil {
		log.Fatal(err)
	}
	if err := echo.Start(); err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	for {
		out := make([]byte, 10)
		n, err := pipe.Read(out) // 读取命令管道内容
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		if n > 0 {
			buf.Write(out[:n])
		}
	}
	fmt.Printf("%q\n", buf.String()) // the quick brown fox jumps over the lazy dog
}
```

此外，模拟 2 个系统命令，可手动设置 `cmd1.Stdout = &buf` 和 `cmd2.Stdin = &buf` 来模拟匿名管道。匿名管道由于交换双方受限的特点，使用场景不多。



## 命名管道

### os.Pipe

`os` 包中支持操作系统级别的命名管道生成与操作：

```go
// os.Pipe 不是阻塞的
func namedPipe() error {
	reader, writer, err := os.Pipe() // 生成命名管道的 reader 和 writer
	if err != nil {
		return err
	}
	_, err = writer.Write([]byte("pipe content")) // writer 写数据
	if err != nil {
		return err
	}

	// writer 只能写，reader 只能读，反向应用将出错
	// r.Write([]byte("test")) // 0: bad file descriptor 

	buf := make([]byte, 20)
	n, err := reader.Read(buf) // reader 读数据
	if err != nil {
		return err
	}
	fmt.Printf("%q\n", string(buf[:n])) // "pipe content"
	return nil
}
```



### io.Pipe

`io` 包中支持线程安全的命名管道生成与原子性操作：
由于命名管道提供多路复用，即多个进程都可向 Pipe 中写入数据，此时需要保证操作互斥，`io.Pipe` 提供了更为安全的原子性操作管道。不过注意它的阻塞操作：

```go
// io.Pipe 命名管道是阻塞的
func atomicPipe() {
	reader, writer := io.Pipe()
    _, err := writer.Write([]byte("content")) // panic: all goroutines are asleep - deadlock
	if err != nil {
		log.Fatal("writer fail: ", err)
	}
	buf := make([]byte, 3)
	_, err = reader.Read(buf)
	if err != nil {
		log.Fatal("read fail: ", err)
	}
}
```
命名管道在一端未就绪的情况下，会阻塞另一端的进程。
上边 writer 写入数据后会一直阻塞直到有进程从 pipe 读取数据，不过其后顺序执行的 reader 读取不可能执行到，才造成死锁

稍微改造下，对命名管道的读写都改成 **并发调用** 即可：

```go
func atomicPipe() {
	reader, writer := io.Pipe()
	go func() {
		if _, err := writer.Write([]byte("content")); err != nil {
			log.Fatal("writer fail: ", err)
		}
	}()
	go func() {
		buf := make([]byte, 3)
		n, err := reader.Read(buf)
		if err != nil {
			log.Fatal("read fail: ", err)
		}
		fmt.Printf("%q\n", buf[:n]) // "con"
	}()
	time.Sleep(1 * time.Second)
}
```

从上可总结出 Golang 中命名管道的 2 个特点：
- os.Pipe 比较底层，不保证多次写入的原子操作，不阻塞。而 io.Pipe 读写操作有独占锁限制，是线程安全的.
- io.Pipe 读写都是阻塞的，应并发读写而非顺序读写。



## io.Pipe 源码分析

源码 `$GOROOT/src/io/pipe.go` 对 pipe 定义如下：

```go
type pipe struct {
	rl    sync.Mutex // 读锁，一次只允许一个 reader 从管道读数据
	wl    sync.Mutex // 写锁，一次只允许一个 writer 向管道写数据
	l     sync.Mutex // 全局锁，配合下边的 sync.Cond 使用
	data  []byte     // 管道内的数据
	rwait sync.Cond  // 等待 reader
	wwait sync.Cond  // 等待 writer
	rerr  error      // 如果 reader 主动 Close 则给 writer 返回 ErrClosedPipe 错误
	werr  error      // 如果 writer 主动 Close 则给 reader 返回 ErrClosedPipe 错误
}
```



`PipeReader` 从管道内读取数据的操作如下：

```go
// 注意返回值 n 和 err，当错误发生时依旧会把成功读取的字节数 n 给返回来
func (p *pipe) read(b []byte) (n int, err error) {
	// 加读锁，保证本次读操作不会被其他 reader 中断
	p.rl.Lock()
	defer p.rl.Unlock()

	p.l.Lock()
	defer p.l.Unlock()
	for {
		if p.rerr != nil {
			return 0, ErrClosedPipe
		}
		if p.data != nil { // 有数据可读
			break
		}
		if p.werr != nil {
			return 0, p.werr
		}
		p.rwait.Wait() // 没有数据读就阻塞等着，直到 writer 通知我管道内有新数据可读了
	}
	n = copy(b, p.data) // 比较关键，直接将 writer 写入的数据 copy 了一份没有用临时内存，减少内存消耗
	p.data = p.data[n:]
	if len(p.data) == 0 {
		p.data = nil
		p.wwait.Signal() // 管道中的数据读完了，通知 writer 可以再往管道写数据了
	}
	return
}
```



`PipeWriter` 将数据写入管道的操作如下：

```go
// 哪怕发生错误也会把成功写入的字节数返回来
func (p *pipe) write(b []byte) (n int, err error) {
	if b == nil {
		b = zero[:]
	}

	// 加写锁，保证本次写操作不会被其他 writer 中断
	p.wl.Lock()
	defer p.wl.Unlock()

	p.l.Lock()
	defer p.l.Unlock()
	if p.werr != nil {
		err = ErrClosedPipe // reader 已经关闭，没必要再写数据了，反正写了又没人读
		return
	}
	p.data = b       // 写数据
	p.rwait.Signal() // 通知 reader 说管道中有数据了
	for {
		if p.data == nil { // 管道内的数据读完了，写操作结束
			break
		}
		if p.rerr != nil {
			err = p.rerr
			break
		}
		if p.werr != nil {
			err = ErrClosedPipe
			break
		}
		p.wwait.Wait() // 一直阻塞等待，等到有 reader 来把管道里边的数据读完，才执行完毕
	}
	n = len(b) - len(p.data)
	p.data = nil // in case of rerr or werr
	return
}
```

注意观察会发现 read 和 write 用到了同一把锁 `p.l`，那先写数据时在死循环内一直死等，`l` 锁也不释放，那 read 获取不到锁怎么读？

实际上是可以读的，关键在于 `p.wwait.Wait()` 中 `Wait` 的实现：

```go
func (c *Cond) Wait() {
	c.checker.check()
	t := runtime_notifyListAdd(&c.notify)
	c.L.Unlock()
	runtime_notifyListWait(&c.notify, t)
	c.L.Lock()
}
```

可看到，在等待时会在内部释放锁，让被通知的 PipeReader 获取锁，才能读取数据。

看其他源码会发现同一个 Pipe 的任意 reader 或 writer 主动 `Close` 掉后，其他端操作时会得到 `ErrClosedPipe` 的错误，但返回值里边待有成功操作的字节数，保证了已操作的数据不丢失。



### 应用场景

分析 io.Pipe 源码可知，PipeWriter 和 PipeReader 通过 pipe 的 `data []byte` 来进行数据传输，其中 lock 机制保证了 Pipe 在同一时刻只能有一个操作，并且 writer 主动写入必须阻塞到有 reader 读取，reader 主动读取必须阻塞到有 writer 写入。

Pipe 给人的感觉就是 2 个 goroutine 一个写一个读，但完全可以用在多写对多读的场景，不过我没使用过。下边的示例将 5 个字符放到 5 个 writer 分别写入到 Pipe，另一端的 reader 一直在等待读取直到最后一个 writer 手动 Close：

```go
// 多个 writer 将数据写入 pipe，一个 reader 读取数据
func main() {
	c := make(chan int)
	r, w := io.Pipe()
	go read(r, c)

	buf := []byte("abcde")
	for i := 0; i < 5; i++ {
		p := buf[i : i+1]
		n, err := w.Write(p) 
		if n != len(p) {
			log.Fatalf("wrote %d, got %d", len(p), n)
		}
		if err != nil {
			log.Fatalf("write: %v", err)
		}
		nn := <-c
		if nn != n {
			log.Fatalf("wrote %d, read got %d", n, nn)
		}
	}

	w.Close() // 发送完毕手动 Close
	nn := <-c
	if nn != 0 {
		log.Fatalf("final read got %d", nn)
	}
}

func read(r io.Reader, c chan int) {
	for {
		var buf = make([]byte, 64)
		n, err := r.Read(buf)
		if err == io.EOF {
			c <- 0
			break
		}
		if err != nil {
			log.Fatalf("read fail: %v", err)
		}
		fmt.Printf("[read]: %s\n", buf[:n])
		c <- n
	}
}
```

运行：

 <img src="https://images.yinzige.com/2019-02-20-102110.png" width=60% />





## 总结

进程间通信方式包括管道，信号，消息队列，共享内存，信号量和 socket 等方式，对应到 Go 实现是 io.Pipe，os.Signal，sync.Mutex，net pkg 等。本文简要解析了 Go 中 pipe 的实现，实际开发中它的场景比较冷门，比如像数据流的实时处理可以考虑使用。

