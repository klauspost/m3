/*
 * Warp (C) 2019-2020 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package bench

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/minio/minio/pkg/console"
)

// Put benchmarks upload speed.
type Put struct {
	Common
	prefixes map[string]struct{}
}

// Prepare will create an empty bucket ot delete any content already there.
func (u *Put) Prepare(ctx context.Context) error {
	return u.createEmptyBucket(ctx)
}

// Start will execute the main benchmark.
// Operations should begin executing when the start channel is closed.
func (u *Put) Start(ctx context.Context, wait chan struct{}) (Operations, error) {
	var wg sync.WaitGroup
	wg.Add(u.Concurrency)
	c := NewCollector()
	if u.AutoTermDur > 0 {
		ctx = c.AutoTerm(ctx, http.MethodPut, u.AutoTermScale, autoTermCheck, autoTermSamples, u.AutoTermDur)
	}
	u.prefixes = make(map[string]struct{}, u.Concurrency)
	for i := 0; i < u.Concurrency; i++ {
		src := u.Source()
		u.prefixes[src.Prefix()] = struct{}{}
		go func(i int) {
			rcv := c.Receiver()
			defer wg.Done()
			opts := u.PutOpts
			done := ctx.Done()

			<-wait
			for {
				select {
				case <-done:
					return
				default:
				}
				obj := src.Object()
				opts.ContentType = obj.ContentType
				client, cldone := u.Client()
				op := Operation{
					OpType:   http.MethodPut,
					Thread:   uint16(i),
					Size:     obj.Size,
					File:     obj.Name,
					ObjPerOp: 1,
					Endpoint: client.EndpointURL().String(),
				}
				op.Start = time.Now()
				n, err := client.PutObject(u.Bucket, obj.Name, obj.Reader, obj.Size, opts)
				op.End = time.Now()
				if err != nil {
					console.Errorln("upload error:", err)
					op.Err = err.Error()
				}
				if n != obj.Size {
					err := fmt.Sprint("short upload. want:", obj.Size, ", got:", n)
					if op.Err == "" {
						op.Err = err
					}
					console.Errorln(err)
				}
				cldone()
				rcv <- op
			}
		}(i)
	}
	wg.Wait()
	return c.Close(), nil
}

// Cleanup deletes everything uploaded to the bucket.
func (u *Put) Cleanup(ctx context.Context) {
	var pf []string
	for p := range u.prefixes {
		pf = append(pf, p)
	}
	u.deleteAllInBucket(ctx, pf...)
}
