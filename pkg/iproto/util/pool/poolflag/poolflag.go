package poolflag

import (
	"bytes"
	"flag"
	"fmt"
	"strconv"
	"strings"
)

/*
// Get returns pool config factory function.
//
// It registers flag flags with given prefix that are necessary
// for config construction.
//
// Call of returned function returns new instance of pool config.
//
// For example:
//	var myPoolConfig = poolflag.Get("my");
//
//	func main() {
//		config := myPoolConfig()
//		p := pool.New(config)
//	}
//
func Get(prefix string) func() *pool.Config {
	return Export(flag.CommandLine, prefix)
}

// GetWithStat returns pool config factory function.
//
// It registers flag flags with given prefix that are necessary
// for config construction. It also registers flags that helps to
// configure statistics measuring.
//
// Call of this function returns new instance of pool config.
//
// Returned config's callback options are filled with stat functions.
//
// Currently, these statistics are measured:
// 	- time of task queued (run_queue_time);
//	- time of task execution (exec_time);
//	- time of workers being idle (workers_idle_time);
//	- count of queued tasks (queued_tasks);
//	- throughput of incoming tasks (task_in);
//	- throughput of tasks performed (task_out);
//	- count of alive workers;
func GetWithStat(prefix string) func(...stat.Tag) *pool.Config {
	return ExportWithStat(flag.CommandLine, prefix)
}

// Export is the same as Get but uses given flag.FlagSet instead of
// flag.CommandLine.
func Export(flag *flag.FlagSet, prefix string) func() *pool.Config {
	return config.Export(flag, prefix)
}

// ExportWithStat is the same as GetWithStat but uses given flag.FlagSet instead
// of flag.CommandLine.
func ExportWithStat(flag *flag.FlagSet, prefix string) func(...stat.Tag) *pool.Config {
	return config.ExportWithStat(
		flagSetWrapper{flag},
		prefix,
	)
}
*/

type flagSetWrapper struct {
	*flag.FlagSet
}

func (f flagSetWrapper) Float64Slice(name string, def []float64, desc string) *[]float64 {
	v := new(float64slice)
	*v = float64slice(def)
	f.Var(v, name, desc)

	return (*[]float64)(v)
}

type float64slice []float64

func (f *float64slice) Set(v string) (err error) {
	var (
		values = strings.Split(v, ",")
		vs     = make([]float64, len(values))
	)

	for i, v := range values {
		vs[i], err = strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return err
		}
	}

	*f = float64slice(vs)

	return nil
}

func (f *float64slice) String() string {
	var buf bytes.Buffer

	for i, f := range *f {
		if i != 0 {
			buf.WriteString(", ")
		}

		fmt.Fprintf(&buf, "%f", f)
	}

	return buf.String()
}
