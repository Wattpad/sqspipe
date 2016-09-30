package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"golang.org/x/net/context"
)

var (
	maxMessages = aws.Int64(10)
	waitTime    = aws.Int64(20)
	Version     = "UNKNOWN"
)

func main() {
	var (
		sourceURL, destinationURL string
		maxIdle                   time.Duration
		nWorkers                  int
	)
	flag.StringVar(&sourceURL, "source", "", "Source queue URL")
	flag.StringVar(&destinationURL, "destination", "", "Destination queue URL")
	flag.DurationVar(&maxIdle, "max_idle", 60*time.Second, "Max idle time before stopping")
	flag.IntVar(&nWorkers, "workers", 1, "Number of workers (1-20)")
	flag.Parse()

	if sourceURL == "" || destinationURL == "" {
		fmt.Printf("Usage: %s -source <queue_name> -destination <queue_name>\n", os.Args[0])
		os.Exit(1)
	}

	if nWorkers <= 0 || nWorkers > 20 {
		fmt.Println("workers must be between 1 and 20")
		os.Exit(1)
	}

	if maxIdle.Seconds() < 1 || maxIdle.Seconds() > 60 {
		fmt.Println("max_idle must be between 1s and 60s")
		os.Exit(1)
	}

	fmt.Printf("Starting sqspipe version %s\n", Version)
	p := &pipe{
		SQS:            sqs.New(session.New(aws.NewConfig().WithMaxRetries(0))),
		MaxIdle:        maxIdle,
		SourceURL:      sourceURL,
		DestinationURL: destinationURL,
	}
	p.Start(nWorkers)
	p.Wait()

	fmt.Println("Done")
}

type pipe struct {
	SQS            *sqs.SQS
	SourceURL      string
	DestinationURL string
	MaxIdle        time.Duration

	remaining int64
	idleTimer *time.Timer
	ctx       context.Context
	wg        sync.WaitGroup
}

func (p *pipe) IsDone() bool {
	if atomic.LoadInt64(&p.remaining) <= 0 {
		fmt.Println("Stopping with 0 remaining")
		return true
	}
	select {
	case <-p.ctx.Done():
		fmt.Printf("Stopping with %d remaining\n", p.NumRemaning())
		return true
	default:
		return false
	}
}

func (p *pipe) DecrRemaining(v int) {
	atomic.AddInt64(&p.remaining, -1*int64(v))
	progressAdd(int64(v))
}

func (p *pipe) NumRemaning() int64 {
	return atomic.LoadInt64(&p.remaining)
}

func (p *pipe) ResetTimer() {
	if !p.idleTimer.Stop() {
		<-p.idleTimer.C
	}
	p.idleTimer.Reset(p.MaxIdle)
}

func (p *pipe) Start(n int) {
	p.remaining = p.countRemainingMessages()
	if p.remaining <= 0 {
		fmt.Println("Queue is empty or unable to read size")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, os.Kill)
	go func() {
		<-term
		log.Println("Starting graceful shutdown (interrupt)")
		cancel()
	}()

	p.idleTimer = time.NewTimer(p.MaxIdle)
	go func() {
		<-p.idleTimer.C
		log.Println("Starting graceful shutdown (timeout)")
		cancel()
	}()

	p.wg.Add(n)
	for i := 0; i < n; i++ {
		go p.runLoop()
	}

	startProgressLogger(ctx)
}

func (p *pipe) Wait() {
	p.wg.Wait()
}

func (p *pipe) runLoop() {
	defer p.wg.Done()
	for !p.IsDone() {
		rmo, err := p.SQS.ReceiveMessage(&sqs.ReceiveMessageInput{
			MaxNumberOfMessages: maxMessages,
			WaitTimeSeconds:     waitTime,
			QueueUrl:            &p.SourceURL,
		})
		if err != nil {
			fmt.Printf("Error receiving messages with %d remaining: %s\n", p.NumRemaning(), err)
			continue
		}

		if len(rmo.Messages) == 0 {
			continue
		}

		p.ResetTimer()
		successes := p.sendMessageBatch(rmo.Messages)

		var entries []*sqs.DeleteMessageBatchRequestEntry
		for _, m := range rmo.Messages {
			if !successes[*m.MessageId] {
				continue
			}
			entries = append(entries, &sqs.DeleteMessageBatchRequestEntry{
				ReceiptHandle: m.ReceiptHandle,
				Id:            m.MessageId,
			})
		}
		if len(entries) == 0 {
			continue
		}

		_, err = p.SQS.DeleteMessageBatch(&sqs.DeleteMessageBatchInput{
			QueueUrl: &p.SourceURL,
			Entries:  entries,
		})
		if err != nil {
			fmt.Printf("Error deleting messages with %d remaining: %s\n", p.NumRemaning(), err)
			continue
		}

		p.DecrRemaining(len(successes))
	}
}

func (p *pipe) countRemainingMessages() int64 {
	qao, err := p.SQS.GetQueueAttributes(&sqs.GetQueueAttributesInput{
		QueueUrl:       &p.SourceURL,
		AttributeNames: []*string{aws.String(sqs.QueueAttributeNameApproximateNumberOfMessages)},
	})
	if err != nil {
		fmt.Printf("Error getting queue size: %s\n", err)
		os.Exit(1)
	}

	if v, ok := qao.Attributes[sqs.QueueAttributeNameApproximateNumberOfMessages]; ok && v != nil {
		if vi, err := strconv.ParseInt(*v, 10, 64); err == nil {
			return vi
		}
	}
	return 0
}

// sendMessageBatch sends a batch of messages and returns the ids of those that succeeded
func (p *pipe) sendMessageBatch(messages []*sqs.Message) map[string]bool {
	bi := &sqs.SendMessageBatchInput{QueueUrl: &p.DestinationURL}

	for _, m := range messages {
		bi.Entries = append(bi.Entries, &sqs.SendMessageBatchRequestEntry{
			Id:          aws.String(*m.MessageId),
			MessageBody: aws.String(*m.Body),
		})
	}

	resp, err := p.SQS.SendMessageBatch(bi)
	if err != nil {
		return nil
	}

	oks := make(map[string]bool)
	if len(resp.Successful) > 0 {
		for _, e := range resp.Successful {
			oks[*e.Id] = true
		}
	}
	return oks
}

var progress int64

func progressAdd(n int64) {
	atomic.AddInt64(&progress, n)
}

func startProgressLogger(ctx context.Context) {
	go func() {
		var lastProgress int64
		lastTime := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(1 * time.Second):
			}

			curProgress := atomic.LoadInt64(&progress)
			curTime := time.Now()
			rate := (float64)(curProgress-lastProgress) / curTime.Sub(lastTime).Seconds()
			lastProgress = curProgress
			lastTime = curTime

			log.Printf("Sent %d messages (%f msgs/sec)", curProgress, rate)
		}
	}()
}
