package main

import (
	"sort"
	"strconv"
	"sync"
)

// сюда писать код

func ExecutePipeline(freeFlowJobs ...job) {
	pool := make([]chan interface{}, 0, len(freeFlowJobs)+1)
	for i := 0; i < len(freeFlowJobs)+1; i++ {
		pool = append(pool, make(chan interface{}, 1))
	}
	wg := &sync.WaitGroup{}
	for index, currentJob := range freeFlowJobs {
		wg.Add(1)
		go runWork(wg, currentJob, pool[index], pool[index+1])
	}
	wg.Wait()
}

func runWork(group *sync.WaitGroup, currentJob job, in, out chan interface{}) {
	defer group.Done()
	currentJob(in, out)
	defer close(out)
}

func runSigner32(in <-chan string, out chan string) {
	for data := range in {
		go func(ch chan string, data string) {
			ch <- DataSignerCrc32(data)
		}(out, data)
	}
}

func runSignerMd5(data string, out chan string, mutex *sync.Mutex) {
	ch := make(chan string, 1)
	tempOut := make(chan string, 1)

	go func(data string, ch chan string) {
		mutex.Lock()
		ch <- DataSignerMd5(data)
		mutex.Unlock()
	}(data, ch)
	go func(ch chan string, tempOut chan string) {
		res := <-ch
		out <- DataSignerCrc32(res)
		close(ch)
	}(ch, tempOut)
}

func SingleHash(in, out chan interface{}) {
	mtx := &sync.Mutex{}
	wg := &sync.WaitGroup{}

	for data := range in {
		wg.Add(1)
		go func(wg *sync.WaitGroup, data interface{}) {
			defer wg.Done()

			md5Out := make(chan string)
			signerOut32 := make(chan string)
			signerIn32 := make(chan string)
			resOut := make(chan string)

			num := data.(int)
			strNum := strconv.Itoa(num)
			go runSignerMd5(strNum, md5Out, mtx)
			go runSigner32(signerIn32, signerOut32)
			signerIn32 <- strNum
			go func(first chan string, second chan string, out chan string) {
				firstR := <-first
				secondR := <-second
				out <- firstR + "~" + secondR
			}(signerOut32, md5Out, resOut)
			res := <-resOut
			out <- res
		}(wg, data)
	}
	wg.Wait()
}

func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	for data := range in {
		hash := data.(string)
		wg.Add(1)
		go func(mainWg *sync.WaitGroup) {
			defer mainWg.Done()
			tempWg := &sync.WaitGroup{}
			hashData := make([]string, 6)
			for i := 0; i < 6; i++ {
				tempWg.Add(1)
				go func(index int, hash string) {
					defer tempWg.Done()
					hashData[index] = DataSignerCrc32(strconv.Itoa(index) + hash)
				}(i, hash)
			}
			tempWg.Wait()
			result := ""
			for i := 0; i < 6; i++ {
				result += hashData[i]
			}
			out <- result
		}(wg)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	resultsHash := make([]string, 0)
	for data := range in {
		resultsHash = append(resultsHash, data.(string))
	}
	sort.Strings(resultsHash)
	resultHash := ""
	for index, hash := range resultsHash {
		resultHash += hash
		if index+1 != len(resultsHash) {
			resultHash += "_"
		}
	}
	out <- resultHash
}
