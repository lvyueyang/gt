package cmd

import (
	"fmt"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/fsnotify/fsnotify"
	"github.com/gookit/color"
	"github.com/spf13/viper"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type Config struct {
	IgnoreDirs  []string // 忽略的目录
	IgnoreFiles []string // 忽略的文件
	Runs        []string //  需要执行的命令
}

var (
	currentPath string
	config      Config
)

// 前置执行
func runBefore() {
	for _, c := range config.Runs {
		if c == "" {
			return
		}
		log.Println(color.Blue.Sprint("Run ", c))
		args := strings.Split(c, " ")
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = currentPath
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			log.Printf("执行命令 %v 失败, 失败原因 %v", c, err)
		}
	}
}

var runCommand *exec.Cmd

func run() {
	if runCommand != nil {
		if err := runCommand.Process.Kill(); err != nil {
			fmt.Printf("关闭失败 %+v", err)
			panic(err)
		}
	}

	//build := exec.Command("go", "build")
	//build.Dir = currentPath
	//build.Stdout = os.Stdout
	//if err := build.Start(); err != nil {
	//	fmt.Println("build 失败")
	//	panic(err)
	//} else {
	//	fmt.Println("build 成功")
	//}

	fmt.Println("开使运行")
	runCommand = exec.Command("go", "run", "main.go")
	runCommand.Dir = currentPath
	runCommand.Stdout = os.Stdout

	if err := runCommand.Start(); err != nil {
		fmt.Println("启动失败")
		panic(err)
	} else {
		fmt.Println("启动成功", runCommand.Process.Pid)
	}
}

func runHandler() {
	runBefore()
	fmt.Println("runHandler")
	run()
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "监听文件变化并执行 go run",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		log.Println("gt!启动！")

		currentPath = getCurrentPath()
		log.Println("监听目录:", currentPath)

		config = getConfig()

		watcher, err := fsnotify.NewWatcher()

		if err != nil {
			log.Fatal(err)
		}

		defer watcher.Close()

		var changeHandler = createDebounce(
			func(event fsnotify.Event) {
				runHandler()
			},
			2000*time.Millisecond,
		)

		// 开始监听
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					// 忽略文件
					ignoreFiles := slice.Map(config.IgnoreFiles, func(index int, item string) string {
						p, _ := filepath.Abs(path.Join(currentPath, item))
						return p
					})

					if !strutil.HasPrefixAny(event.Name, ignoreFiles) {
						// 判断是否是一个文件路径
						if isFile(event.Name) {
							fmt.Println("文件发生变化: ", event.Name)
							changeHandler(event)
						}
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					log.Println("error:", err)
				}
			}
		}()

		// 递归监听路径
		err = watchDir(watcher, currentPath)
		if err != nil {
			log.Fatal(err)
		}

		runHandler()

		<-make(chan struct{})
	},
}

// 创建一个防抖方法
func createDebounce[T any](f func(v T), duration time.Duration) func(v T) {
	var timer *time.Timer

	return func(v T) {
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(duration, func() {
			f(v)
		})
	}
}

// 获取执行目录
func getCurrentPath() string {
	currentDirectory, err := os.Getwd()
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}
	return currentDirectory
}

// 获取配置信息
func getConfig() Config {
	file, _ := filepath.Abs(path.Join(currentPath, "gtconfig.toml"))
	viper.SetConfigFile(file)
	viper.SetConfigType("toml")

	//判断文件是否存在
	if err := viper.ReadInConfig(); err != nil {
		return Config{
			IgnoreDirs:  []string{".git", ".idea"},
			IgnoreFiles: []string{},
			Runs:        []string{},
		}
	}

	var conf Config

	if err := viper.Unmarshal(&conf); err != nil {
		fmt.Printf("配置映射失败 %+v", err)
	}
	viper.WatchConfig()
	log.Printf("配置信息: %+v\n", conf)
	return conf
}

func watchDir(watcher *fsnotify.Watcher, dir string) error {
	ignore := slice.Map(config.IgnoreDirs, func(index int, item string) string {
		p, _ := filepath.Abs(path.Join(dir, item))
		return p
	})
	fmt.Println("IGNORE:", ignore)

	// 遍历子目录
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		// 判断是否为目录，只需要监听目录，目录下的文件变化就可以进行相应操作
		if info.IsDir() {

			p, err := filepath.Abs(p)
			if err != nil {
				log.Println(err)
				return err
			}
			// 判断是否需要忽略
			if !strutil.HasPrefixAny(p, ignore) {
				// 然后将其加入监听
				if err := watcher.Add(p); err != nil {
					log.Printf("监听目录 %s 错误. 错误信息 = %s", p, err.Error())
					return err
				} else {
					log.Println("正在监听目录:", p)
				}
			}
		}
		return nil
	})
	return nil
}

func isFile(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func init() {
	rootCmd.AddCommand(runCmd)
}
