package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sj14/dbbench/databases"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// Bencher is the interface a benchmark has to impelement.
type Bencher interface {
	Setup(...string)
	Cleanup()
	Benchmarks() []databases.Benchmark
}

var (
	app      = kingpin.New("chat", "A command-line chat application.")
	debug    = app.Flag("debug", "Enable debug mode.").Bool()
	serverIP = app.Flag("server", "Server address.").Default("127.0.0.1").IP()
	asd      = app.Flag("asd", "asdsd").Hidden()
)

func main() {
	// TODO: add database name flag (especially for mysql)
	// global flags
	db := flag.String("db", "", "database to use (sqlite|mariadb|mysql|postgres|cockroach|cassandra|scylla)")
	host := flag.String("host", "localhost", "address of the server")
	port := flag.Int("port", 0, "port of the server")
	user := flag.String("user", "root", "user name to connect with the server")
	pass := flag.String("pass", "root", "password to connect with the server")
	conns := flag.Int("conns", 0, "max. number of open connections")

	iter := flag.Int("iter", 1000, "how many iterations should be run")
	threads := flag.Int("threads", 25, "max. number of green threads")
	clean := flag.Bool("clean", false, "only cleanup previous benchmark data, e.g. due to a crash")
	noclean := flag.Bool("noclean", false, "dont cleanup benchmark data")
	// version := flag.Bool("version", false, "print version information") // TODO
	runBench := flag.String("run", "all", "only run the specified benchmarks, e.g. \"inserts deletes\"") // TODO

	// subcommands and local flags
	// cassandra := flag.NewFlagSet("cassandra", flag.ExitOnError)
	flag.Parse()

	bencher := getImpl(*db, *host, *port, *user, *pass, *conns)

	// only clean old data when clean flag is set
	if *clean {
		bencher.Cleanup()
		os.Exit(0)
	}

	// setup database
	bencher.Setup()

	// only cleanup benchmark data when noclean flag is not set
	if !*noclean {
		defer bencher.Cleanup()
	}

	// split benchmark names when "-run 'bench0 bench1 ...'" flag was used
	toRun := strings.Split(*runBench, " ")

	start := time.Now()
	for _, r := range toRun {
		benchmark(bencher, r, *iter, *threads)
	}
	fmt.Printf("total: %v\n", time.Since(start))
}

func getImpl(dbType string, host string, port int, user, password string, maxOpenConns int) Bencher {
	switch dbType {
	case "sqlite":
		if maxOpenConns != 0 {
			log.Fatalln("can't use 'conns' with SQLite")
		}
		return databases.NewSQLite()
	case "mysql", "mariadb":
		return databases.NewMySQL(host, port, user, password, maxOpenConns)
	case "postgres":
		return databases.NewPostgres(host, port, user, password, maxOpenConns)
	case "cockroach":
		return databases.NewCockroach(host, port, user, password, maxOpenConns)
	case "cassandra", "scylla":
		if maxOpenConns != 0 {
			log.Fatalln("can't use 'conns' with Cassandra or ScyllaDB")
		}
		return databases.NewCassandra(host, port, user, password)
	}

	log.Fatalln("missing or unknown type parameter")
	return nil
}

func benchmark(bencher Bencher, runBench string, iterations, goroutines int) {
	wg := &sync.WaitGroup{}
	// w := tabwriter.NewWriter(os.Stdout, 0, 4, 4, '\t', tabwriter.AlignRight)

	for _, b := range bencher.Benchmarks() {
		// check if we want to run this particular benchmark
		if runBench != "all" && b.Name != runBench {
			continue
		}

		wg.Add(goroutines)
		start := time.Now()
		for i := 0; i < goroutines; i++ {
			from := (iterations / goroutines) * i
			to := (iterations / goroutines) * (i + 1)

			go func() {
				defer wg.Done()
				for i := from; i < to; i++ {
					b.Func(i)
				}
			}()
		}
		wg.Wait()
		elapsed := time.Since(start)
		fmt.Printf("%v\t%v\t%v\tns/op\n", b.Name, elapsed, elapsed.Nanoseconds()/int64(iterations))
		// fmt.Fprintf(w, "%v\t%v\t%v\tns/op\n", name, elapsed, elapsed.Nanoseconds()/int64(iterations))
	}
	// w.Flush()
}
