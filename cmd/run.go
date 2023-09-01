package cmd

import (
	"fmt"
	"github.com/duke-git/lancet/v2/slice"
	"github.com/duke-git/lancet/v2/strutil"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

type Config struct {
	IgnoreDirs []string
}

var (
	currentPath string
	config      Config
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "监听文件变化并执行 go run",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("gt!启动！")

		currentPath = getCurrentPath()
		fmt.Println("监听目录:", currentPath)

		config = getConfig()

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatal(err)
		}
		defer watcher.Close()

		var changeHandler = createDebounce[string](
			func(name string) {
				log.Println("文件变化:", name)
			},
			1000*time.Millisecond,
		)

		// 开始监听
		go func() {
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					log.Println("事件:", event)
					changeHandler(event.Name)
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

		<-make(chan struct{})
	},
}

func createDebounce[T any](handle func(T), duration time.Duration) func(T) {
	var lastFire time.Time
	return func(v T) {
		now := time.Now()
		if now.Sub(lastFire) < duration {
			return
		}
		lastFire = now
		handle(v)
	}
}

func getCurrentPath() string {
	currentDirectory, err := os.Getwd()
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}
	return currentDirectory
}

func getConfig() Config {
	file, _ := filepath.Abs(path.Join(currentPath, "gtconfig.toml"))
	viper.SetConfigFile(file)
	viper.SetConfigType("toml")

	//判断文件是否存在
	if err := viper.ReadInConfig(); err != nil {
		return Config{
			IgnoreDirs: []string{".git", ".idea"},
		}
	}

	if err := viper.ReadInConfig(); err != nil {
		panic("配置文件不存在")
	}

	var conf Config

	viper.Unmarshal(&conf)
	viper.WatchConfig()
	log.Printf("配置信息: %+v\n", conf)
	return conf
}

type Throttler struct {
	maxRate int        // 最大调用频率，比如每秒最多调用多少次
	last    time.Time  // 上次调用的时间
	lock    sync.Mutex // 用于保护节流器的互斥锁
}

func (t *Throttler) Throttle(funcToCall func()) {
	t.lock.Lock()
	defer t.lock.Unlock()

	now := time.Now()
	elapsed := now.Sub(t.last) // 计算自上次调用以来的时间差
	if elapsed < time.Second/time.Duration(t.maxRate) {
		time.Sleep(time.Second/time.Duration(t.maxRate) - elapsed) // 等待一段时间再尝试
	}
	t.last = now // 更新上次调用时间
	funcToCall() // 执行函数
}

func watchDir(watcher *fsnotify.Watcher, dir string) error {
	ignore := slice.Map(config.IgnoreDirs, func(index int, item string) string {
		p, _ := filepath.Abs(path.Join(dir, item))
		return p
	})

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

func init() {
	rootCmd.AddCommand(runCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// runCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// runCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
