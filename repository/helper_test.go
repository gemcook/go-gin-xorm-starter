package repository_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/gemcook/go-gin-xorm-starter/infra"
	"github.com/go-xorm/xorm"
)

var dockerMySQLImage = "mysql:5.7.21"
var dockerMySQLPort = "11336"

// Setup initializes test environment.
// Call cleanup func with 'defer'.
func Setup(t *testing.T) (engine *xorm.Engine, cleanup func()) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.SkipNow()
	}

	dockerInfoCmd := exec.Command("docker", "info")
	err := dockerInfoCmd.Run()
	if err != nil {
		t.Skipf("docker daemon is not running. error=%v", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Chdir("..")
	if err != nil {
		t.Fatal(err)
	}

	dockerRunCmd := exec.Command("docker", "container", "run",
		"--rm",
		"-p", dockerMySQLPort+":3306",
		"-e", "MYSQL_ROOT_PASSWORD=password",
		"-e", "TZ=Asia/Tokyo",
		dockerMySQLImage)
	rc, err := dockerRunCmd.StdoutPipe()
	if err == nil {
		go func() {
			io.Copy(os.Stdout, rc)
		}()
	}

	err = dockerRunCmd.Start()
	if err != nil {
		t.Fatal(err)
	}

	setMySQLTestEnv()
	initDatabase(t)

	engine, err = infra.InitMySQLEngine(infra.LoadMySQLConfigEnv())
	if err != nil {
		t.Fatal(err)
	}
	engine.ShowSQL(false)
	engine.SetConnMaxLifetime(time.Second)

	// clean up function.
	return engine, func() {
		if t.Skipped() {
			return
		}

		defer os.Chdir(currentDir)
		engine.Close()

		// send interrupt signal to docker command.
		err = dockerRunCmd.Process.Signal(syscall.SIGINT)
		if err != nil {
			fmt.Println("SIGINT:", err)
		}

		err = dockerRunCmd.Wait()
		if err != nil {
			fmt.Println("dockerRunCmd.Wait()", err)
		}

		dockerRmImageCmd := exec.Command("sh", "-c", fmt.Sprintf(`'docker container rm -f $(docker container ps -q -f "ancestor=%s")'`, dockerMySQLImage))

		err = dockerRmImageCmd.Run()
		if err != nil {
			fmt.Println(err)
		}
	}
}

func setMySQLTestEnv() {
	os.Setenv("DATABASE_HOST", "localhost:"+dockerMySQLPort)
	os.Setenv("DATABASE_NAME", "go_gin_xorm_starter")
	os.Setenv("DATABASE_USER", "root")
	os.Setenv("DATABASE_PASSWORD", "password")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("LOG_DIR", "log/test")
}

func initDatabase(t *testing.T) {
	mysqlConf := infra.LoadMySQLConfigEnv()
	mysqlConf.DBName = ""
	connStr := mysqlConf.FormatDSN()
	err := infra.RunSQLFile(connStr, "./fixtures/db.sql")
	if err != nil {
		t.Fatal(err)
	}
}
