package tests

import (
	"fmt"
	"github.com/blablacar/dgr/bin-dgr/common"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const DefaultPort = 2181

type ErrMissingServerConfigField string

func (e ErrMissingServerConfigField) Error() string {
	return fmt.Sprintf("zk: missing server config field '%s'", string(e))
}

const (
	DefaultServerTickTime                 = 2000
	DefaultServerInitLimit                = 10
	DefaultServerSyncLimit                = 5
	DefaultServerAutoPurgeSnapRetainCount = 3
	DefaultPeerPort                       = 2888
	DefaultLeaderElectionPort             = 3888
)

type ServerConfigServer struct {
	ID                 int
	Host               string
	PeerPort           int
	LeaderElectionPort int
}

type ServerConfig struct {
	TickTime                 int    // Number of milliseconds of each tick
	InitLimit                int    // Number of ticks that the initial synchronization phase can take
	SyncLimit                int    // Number of ticks that can pass between sending a request and getting an acknowledgement
	DataDir                  string // Direcrory where the snapshot is stored
	ClientPort               int    // Port at which clients will connect
	AutoPurgeSnapRetainCount int    // Number of snapshots to retain in dataDir
	AutoPurgePurgeInterval   int    // Purge task internal in hours (0 to disable auto purge)
	Servers                  []ServerConfigServer
}

func (sc ServerConfig) Marshall(w io.Writer) error {
	if sc.DataDir == "" {
		return ErrMissingServerConfigField("dataDir")
	}
	fmt.Fprintf(w, "dataDir=%s\n", sc.DataDir)
	if sc.TickTime <= 0 {
		sc.TickTime = DefaultServerTickTime
	}
	fmt.Fprintf(w, "tickTime=%d\n", sc.TickTime)
	if sc.InitLimit <= 0 {
		sc.InitLimit = DefaultServerInitLimit
	}
	fmt.Fprintf(w, "initLimit=%d\n", sc.InitLimit)
	if sc.SyncLimit <= 0 {
		sc.SyncLimit = DefaultServerSyncLimit
	}
	fmt.Fprintf(w, "syncLimit=%d\n", sc.SyncLimit)
	if sc.ClientPort <= 0 {
		sc.ClientPort = DefaultPort
	}
	fmt.Fprintf(w, "clientPort=%d\n", sc.ClientPort)
	if sc.AutoPurgePurgeInterval > 0 {
		if sc.AutoPurgeSnapRetainCount <= 0 {
			sc.AutoPurgeSnapRetainCount = DefaultServerAutoPurgeSnapRetainCount
		}
		fmt.Fprintf(w, "autopurge.snapRetainCount=%d\n", sc.AutoPurgeSnapRetainCount)
		fmt.Fprintf(w, "autopurge.purgeInterval=%d\n", sc.AutoPurgePurgeInterval)
	}
	if len(sc.Servers) > 0 {
		for _, srv := range sc.Servers {
			if srv.PeerPort <= 0 {
				srv.PeerPort = DefaultPeerPort
			}
			if srv.LeaderElectionPort <= 0 {
				srv.LeaderElectionPort = DefaultLeaderElectionPort
			}
			fmt.Fprintf(w, "server.%d=%s:%d:%d\n", srv.ID, srv.Host, srv.PeerPort, srv.LeaderElectionPort)
		}
	}
	return nil
}

var jarSearchPaths = []string{
	jarPath,
	//"/home/n0rad/Downloads/zookeeper-3.4.8/contrib/fatjar/zookeeper-*-fatjar.jar",
	//"zookeeper-*/contrib/fatjar/zookeeper-*-fatjar.jar",
	//"../zookeeper-*/contrib/fatjar/zookeeper-*-fatjar.jar",
	//"/usr/share/java/zookeeper-*.jar",
	//"/usr/local/zookeeper-*/contrib/fatjar/zookeeper-*-fatjar.jar",
	//"/usr/local/Cellar/zookeeper/*/libexec/contrib/fatjar/zookeeper-*-fatjar.jar",
}

func findZookeeperFatJar() string {
	var paths []string
	zkPath := os.Getenv("ZOOKEEPER_PATH")
	if zkPath == "" {
		paths = jarSearchPaths
	} else {
		paths = []string{filepath.Join(zkPath, "contrib/fatjar/zookeeper-*-fatjar.jar")}
	}
	for _, path := range paths {
		matches, _ := filepath.Glob(path)
		// TODO: could sort by version and pick latest
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

const version = "3.4.8"
const filename = "zookeeper-" + version + ".tar.gz"
const zooTarPath = "/tmp/" + filename
const jarPath = "/tmp/zookeeper-" + version + "/contrib/fatjar/zookeeper-*-fatjar.jar"

func downloadZookeeper() error {
	println("Downloading zookeeper")
	out, err := os.Create(zooTarPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get("http://apache.mirrors.ovh.net/ftp.apache.org/dist/zookeeper/zookeeper-" + version + "/" + filename)
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

type Server struct {
	JarPath        string
	ConfigPath     string
	Stdout, Stderr io.Writer

	cmd *exec.Cmd
}

func (srv *Server) Start() error {
	if _, err := os.Stat(zooTarPath); err != nil {
		if err := downloadZookeeper(); err != nil {
			return err
		}
		if err := common.ExecCmd("tar", "xf", zooTarPath, "-C", "/tmp"); err != nil {
			os.Remove(zooTarPath)
			return err
		}
	}

	if srv.JarPath == "" {
		srv.JarPath = findZookeeperFatJar()
		if srv.JarPath == "" {
			return fmt.Errorf("zk: unable to find server jar")
		}
	}
	srv.cmd = exec.Command("java", "-jar", srv.JarPath, "server", srv.ConfigPath)
	srv.cmd.Stdout = srv.Stdout
	srv.cmd.Stderr = srv.Stderr
	return srv.cmd.Start()
}

func (srv *Server) Stop() error {
	println("Stopping zookeeper server")
	srv.cmd.Process.Signal(os.Kill)
	return srv.cmd.Wait()
}
