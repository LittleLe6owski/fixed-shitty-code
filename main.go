package main

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

// ЗАДАНИЕ:
// * сделать из плохого кода хороший;
// * важно сохранить логику появления ошибочных тасков;
// * сделать правильную мультипоточность обработки заданий.
// * обновленный код отправить через pull-request.

// Приложение эмулирует получение и обработку тасков, пытается и получать и обрабатывать в многопоточном режиме.
// Должно выводить успешные таски и ошибки по мере выполнения.
// Как видите, никаких привязок к внешним сервисам нет - полный карт-бланш на модификацию кода.

// A Ttype represents a meaninglessness of our life
type taskParams struct {
	id  int
	cT  time.Time // время создания
	fT  time.Time // время выполнения
	err error
}

// TaskError - struct for implement Error interface, struct for handle task errors
type TaskError struct {
	Message string
}

// Error - func for implement interface Error, func for formatting text error
func (e *TaskError) Error() string {
	return fmt.Sprintf("task error: %s", e.Message)
}

var (
	// OutdatedExecutionTime - error of incorrect exec task date
	OutdatedExecutionTime = &TaskError{Message: "time to execute task has occurred"}
)

const (
	sortWorkerCount = 3 // волшебная цифра
)

func main() {

	taskChan := make(chan taskParams, 50)
	doneTaskChan := make(chan taskParams)
	undoneTaskChan := make(chan error)

	group, ctx := errgroup.WithContext(context.Background())

	group.Go(
		func() error {
			defer close(taskChan)
			for i := 0; i < 50; i++ { // создадим 50 задач
				select {
				case taskChan <- taskCreator():
				case <-ctx.Done():
					return ctx.Err()
				default:
					time.Sleep(time.Millisecond * 10) // чтобы не напрягать cpu
				}
			}
			fmt.Println("All tasks created")
			return nil
		},
	)

	for i := 0; i < sortWorkerCount; i++ {
		group.Go(
			func() error {
				for {
					select {
					case params, ok := <-taskChan:
						if !ok {
							return errors.New("task channel closed")
						}
						if sortedParams, isDone := sortTaskWorker(params); isDone {
							doneTaskChan <- sortedParams
						} else {
							undoneTaskChan <- sortedParams.err
						}
					case <-ctx.Done():
						return ctx.Err()
					}
				}
			},
		)
	}

	group.Go(
		func() error {
			for {
				select {
				case doneTask, ok := <-doneTaskChan:
					if !ok {
						return errors.New("task channel closed")
					}
					fmt.Printf("Task completed %d \n", doneTask.id)
				case err, ok := <-undoneTaskChan:
					if !ok {
						return errors.New("task channel closed")
					}
					fmt.Printf("Task completed unsuccessfully error: %s \n", err.Error())
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		},
	)

	wg := new(sync.WaitGroup)
	wg.Add(1)

	go func(ctx context.Context, wg *sync.WaitGroup) {
		defer func() {
			ctx.Done()
			wg.Done()
		}()
		time.Sleep(time.Second * 2)
	}(ctx, wg)

	wg.Wait()

	if err := group.Wait(); err != nil {
		fmt.Println(err)
	}

	close(doneTaskChan)
	close(undoneTaskChan)
}

func taskCreator() taskParams {
	ft := time.Now()
	if ft.Nanosecond()%2 > 0 { // вот такое условие появления ошибочных тасков
		fmt.Println("Some error occurred")
	}
	time.Sleep(time.Millisecond * 25)
	return taskParams{cT: ft, id: int(time.Now().Unix())} // передаем таск на выполнение
}

func sortTaskWorker(params taskParams) (taskParams, bool) {

	params.fT = time.Now()
	if !params.cT.IsZero() && !params.cT.After(time.Now().Add(-20*time.Second)) {
		params.err = OutdatedExecutionTime
		return params, false
	}

	return params, true
}
