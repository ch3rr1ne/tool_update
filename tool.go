package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// downloadFile 通用下载函数，包含重试功能
func downloadFile(dlURL, downloadPath string) error {
	// 设置最大重试次数为5
	maxRetries := 5

	for retries := 0; retries <= maxRetries; retries++ {
		if retries > 0 {
			fmt.Printf("第 %d 次重试下载: %s\n", retries, dlURL)
			// 使用指数退避策略
			waitTime := time.Duration(math.Pow(2, float64(retries))) * time.Second
			fmt.Printf("等待 %v 后重试...\n", waitTime)
			time.Sleep(waitTime)
		}

		// 创建HTTP客户端，设置超时
		client := &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// 允许最多10次重定向
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		// 创建请求
		req, err := http.NewRequest("GET", dlURL, nil)
		if err != nil {
			fmt.Printf("创建请求失败: %v\n", err)
			continue
		}

		// 添加用户代理头
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		// 执行下载
		fmt.Printf("开始下载: %s\n", dlURL)
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("下载失败: %v\n", err)
			continue
		}

		// 检查状态码
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			fmt.Printf("下载失败，HTTP状态码: %d\n", resp.StatusCode)
			continue
		}

		// 创建输出文件
		outFile, err := os.Create(downloadPath)
		if err != nil {
			resp.Body.Close()
			fmt.Printf("创建文件失败: %v\n", err)
			continue
		}

		// 复制内容
		bytesWritten, err := io.Copy(outFile, resp.Body)
		resp.Body.Close()
		outFile.Close()

		if err != nil {
			fmt.Printf("写入文件失败: %v\n", err)
			continue
		}

		fmt.Printf("下载成功: %s (大小: %d 字节)\n", downloadPath, bytesWritten)
		return nil // 下载成功
	}

	// 如果达到这里，说明所有重试都失败了
	return fmt.Errorf("下载失败，已达到最大重试次数")
}

// needDownload 检查是否需要下载新版本，并处理旧文件迁移
func needDownload(name, version, versionFile string, savename string, downloadDir string) bool {
    // 检查版本文件是否存在
    if _, err := os.Stat(versionFile); os.IsNotExist(err) {
        return true
    }

    // 读取版本文件，查找已保存的版本号
    file, err := os.Open(versionFile)
    if err != nil {
        return true
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    prefix := name + " = "
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, prefix) {
            // 获取版本部分 - 注意要取第3个空格之后的所有内容
            parts := strings.SplitN(line, " ", 3)
            var savedVersion string
            if len(parts) >= 3 {
                savedVersion = parts[2]
            } else {
                savedVersion = strings.TrimPrefix(line, prefix)
            }
            
            fmt.Printf("比较版本 - 已保存: '%s', 最新: '%s'\n", savedVersion, version)
            
            // 如果版本不同，需要移动旧文件到history目录
            if savedVersion != version {
                fmt.Printf("%s 需要更新: %s -> %s\n", name, savedVersion, version)
                
                // 检查旧版本文件是否存在
                oldFilePath := filepath.Join(downloadDir, savename)
                if _, err := os.Stat(oldFilePath); err == nil {
                    // 确保history目录存在
                    historyDir := filepath.Join(downloadDir, "history")
                    if _, err := os.Stat(historyDir); os.IsNotExist(err) {
                        os.MkdirAll(historyDir, 0755)
                    }
                    
                    // 移动文件到history目录
                    newPath := filepath.Join(historyDir, savename)
                    fmt.Printf("移动旧文件: %s -> %s\n", oldFilePath, newPath)
                    
                    // 如果history目录中已存在同名文件，可以重命名
                    if _, err := os.Stat(newPath); err == nil {
                        timestamp := time.Now().Format("20060102150405")
                        newPath = filepath.Join(historyDir, fmt.Sprintf("%s_%s", timestamp, savename))
                    }
                    
                    err := os.Rename(oldFilePath, newPath)
                    if err != nil {
                        fmt.Printf("移动旧文件失败: %v\n", err)
                    } else {
                        fmt.Printf("成功移动旧版本文件到历史目录\n")
                    }
                    
                    // 清理旧文件，保留最近5个版本
                    pattern := filepath.Join(historyDir, strings.Replace(savename, ".", "\\.", -1)+"*")
                    matches, err := filepath.Glob(pattern)
                    if err == nil && len(matches) > 5 {
                        // 按修改时间排序
                        sort.Slice(matches, func(i, j int) bool {
                            infoI, _ := os.Stat(matches[i])
                            infoJ, _ := os.Stat(matches[j])
                            return infoI.ModTime().After(infoJ.ModTime())
                        })
                        
                        // 删除最旧的文件
                        for i := 5; i < len(matches); i++ {
                            fmt.Printf("删除过旧版本: %s\n", matches[i])
                            os.Remove(matches[i])
                        }
                    }
                } else {
                    fmt.Printf("旧文件不存在: %s\n", oldFilePath)
                }
                return true // 版本不同，需要下载
            }
            return false // 版本相同，不需要下载
        }
    }

    // 如果找不到该工具的版本记录，则需要下载
    fmt.Printf("找不到 %s 的版本记录，需要下载\n", name)
    return true
}

// updateVersionFile 更新版本信息
func updateVersionFile(name, version string, versionFile string) error {
	// 读取现有内容
	var lines []string
	if _, err := os.Stat(versionFile); !os.IsNotExist(err) {
		file, err := os.Open(versionFile)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(file)
		found := false
		prefix := name + " = "
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, prefix) {
				lines = append(lines, prefix+version)
				found = true
			} else {
				lines = append(lines, line)
			}
		}
		file.Close()

		if !found {
			lines = append(lines, prefix+version)
		}
	} else {
		// 如果文件不存在，创建新条目
		lines = append(lines, name+" = "+version)
	}

	// 写回文件
	file, err := os.Create(versionFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		fmt.Fprintln(writer, line)
	}
	return writer.Flush()
}

func main() {
	// 设置下载和版本信息存储路径
	currentDir, _ := os.Getwd()
	downloadDir := filepath.Join(currentDir, "tool")
	historyDir := filepath.Join(downloadDir, "history")
	versionFile := filepath.Join(downloadDir, "version.txt")

	// 确保下载目录和历史目录存在
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		os.MkdirAll(downloadDir, 0755)
	}
	if _, err := os.Stat(historyDir); os.IsNotExist(err) {
		os.MkdirAll(historyDir, 0755)
	}
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		os.Create(versionFile)
	}

	var name, dlURL, getVersion, savename string

	// 下载MSIAfterburner
	name = "MSIAfterburner"
	fmt.Println("开始下载", name)

	// 发送HTTP请求获取MSIAfterburner下载页面
	resp, err := http.Get("https://www.guru3d.com/download/msi-afterburner-beta-download/")
	if err != nil {
		fmt.Println("Error fetching MSIAfterburner page:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading MSIAfterburner response:", err)
		return
	}
	bodyString := string(bodyBytes)

	// 提取版本号文本，格式如: MSI Afterburner 4.6.6 Beta 5 Build 16555
	re := regexp.MustCompile(`MSI Afterburner ([\d\.]+ Beta \d+ Build \d+)`)
	matches := re.FindStringSubmatch(bodyString)

	if len(matches) > 1 {
		fullVersionText := matches[1]
		fmt.Printf("找到MSIAfterburner完整版本: %s\n", fullVersionText)
		
		// 提取纯版本号 (如 4.6.6)
		reVersion := regexp.MustCompile(`([\d\.]+)`)
		versionMatches := reVersion.FindStringSubmatch(fullVersionText)
		
		// 提取Beta版本 (如 5)
		reBeta := regexp.MustCompile(`Beta (\d+)`)
		betaMatches := reBeta.FindStringSubmatch(fullVersionText)
		
		// 提取Build号 (如 16555)
		reBuild := regexp.MustCompile(`Build (\d+)`)
		buildMatches := reBuild.FindStringSubmatch(fullVersionText)
		
		if len(versionMatches) > 1 && len(betaMatches) > 1 && len(buildMatches) > 1 {
			version := versionMatches[1]
			beta := betaMatches[1]
			build := buildMatches[1]
			
			// 构建版本字符串
			getVersion := fmt.Sprintf("%s Beta %s Build %s", version, beta, build)
			fmt.Printf("解析MSIAfterburner版本: %s\n", getVersion)
			
			// 移除版本号中的点和空格，构建文件名的一部分
			versionNoPoints := strings.ReplaceAll(version, ".", "")
			dlVersionPart := fmt.Sprintf("%sBeta%sBuild%s", versionNoPoints, beta, build)
			fmt.Printf("构建的版本字符串: %s\n", dlVersionPart)
			
			// 构建下载URL
			dlURL = fmt.Sprintf("https://ftp.nluug.nl/pub/games/PC/guru3d/afterburner/[Guru3D]-MSIAfterburnerSetup%s.zip", dlVersionPart)
			savename = fmt.Sprintf("[Guru3D]-MSIAfterburnerSetup%s.zip", dlVersionPart)
			
			// 检查是否需要下载
			if needDownload(name, getVersion, versionFile, savename, downloadDir) {
				downloadPath := filepath.Join(downloadDir, savename)
				
				// 使用通用下载函数
				err := downloadFile(dlURL, downloadPath)
				if err != nil {
					fmt.Printf("%s下载失败: %v\n", name, err)
				} else {
					// 更新版本信息
					updateVersionFile(name, getVersion, versionFile)
					fmt.Printf("%s下载完成\n", name)
				}
			} else {
				fmt.Printf("%s已是最新版本\n", name)
			}
		} else {
			fmt.Println("无法解析MSIAfterburner版本详情")
		}
	} else {
		fmt.Println("无法提取MSIAfterburner版本号")
	}

	// 下载CPU-Z
	name = "CPU-Z"
	fmt.Println("开始下载", name)

	// 获取CPU-Z的HTML内容以提取版本号
	resp, err = http.Get("https://www.cpuid.com/softwares/cpu-z.html")
	if err != nil {
		fmt.Println("Error fetching CPU-Z page:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading CPU-Z response:", err)
		return
	}
	bodyString = string(bodyBytes)

	// 提取版本号
	re = regexp.MustCompile(`Version (\d+\.\d+)`)
	matches = re.FindStringSubmatch(bodyString)

	if len(matches) > 1 {
		getVersion = matches[1]
		fmt.Printf("找到CPU-Z版本: %s\n", getVersion)

		dlURL = fmt.Sprintf("https://download.cpuid.com/cpu-z/cpu-z_%s-cn.exe", getVersion)
		savename = filepath.Base(dlURL)

		// 检查是否需要下载
		if needDownload(name, getVersion, versionFile, savename, downloadDir) {
			downloadPath := filepath.Join(downloadDir, savename)
			
			// 使用通用下载函数
			err := downloadFile(dlURL, downloadPath)
			if err != nil {
				fmt.Printf("%s下载失败: %v\n", name, err)
			} else {
				// 更新版本信息
				updateVersionFile(name, getVersion, versionFile)
				fmt.Printf("%s下载完成\n", name)
			}
		} else {
			fmt.Printf("%s已是最新版本\n", name)
		}
	} else {
		fmt.Println("无法提取CPU-Z版本号")
	}

	// 下载GPU-Z
	name = "GPU-Z"
	fmt.Println("开始下载", name)

	// 获取GPU-Z的HTML内容以提取版本号
	resp, err = http.Get("https://www.guru3d.com/download/gpu-z-download-techpowerup/")
	if err != nil {
		fmt.Println("Error fetching GPU-Z page:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading GPU-Z response:", err)
		return
	}
	bodyString = string(bodyBytes)

	// 提取版本号
	re = regexp.MustCompile(`<title>.*?v([\d\.]+).*?</title>`)
	matches = re.FindStringSubmatch(bodyString)

	if len(matches) > 1 {
		getVersion = matches[1]
		fmt.Printf("找到GPU-Z版本: %s\n", getVersion)

		dlURL = "https://ftp.nluug.nl/pub/games/PC/guru3d/generic/GPU-Z-[Guru3D.com].zip"
		filename := filepath.Base(dlURL)
		savename = strings.Replace(filename, "[Guru3D.com]", getVersion, 1)

		// 检查是否需要下载
		if needDownload(name, getVersion, versionFile, savename, downloadDir) {
			downloadPath := filepath.Join(downloadDir, savename)
			
			// 使用通用下载函数
			err := downloadFile(dlURL, downloadPath)
			if err != nil {
				fmt.Printf("%s下载失败: %v\n", name, err)
			} else {
				// 更新版本信息
				updateVersionFile(name, getVersion, versionFile)
				fmt.Printf("%s下载完成\n", name)
			}
		} else {
			fmt.Printf("%s已是最新版本\n", name)
		}
	} else {
		fmt.Println("无法提取GPU-Z版本号")
	}

	// 下载HWINFO
	name = "HWINFO"
	fmt.Println("开始下载", name)

	// 获取HWINFO的HTML内容以提取版本号
	resp, err = http.Get("https://www.hwinfo.com/download/")
	if err != nil {
		fmt.Println("Error fetching HWINFO page:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading HWINFO response:", err)
		return
	}
	bodyString = string(bodyBytes)

	// 提取版本号
	re = regexp.MustCompile(`<sub>Version ([\d\.]+)</sub>`)
	matches = re.FindStringSubmatch(bodyString)

	if len(matches) > 1 {
		getVersion = matches[1]
		fmt.Printf("找到HWINFO版本: %s\n", getVersion)

		// 移除版本号中的点
		getVersionNoPoints := strings.ReplaceAll(getVersion, ".", "")
		fmt.Printf("处理后的HWINFO版本号: %s\n", getVersionNoPoints)

		// 使用新的下载URL
		dlURL = fmt.Sprintf("https://www.sac.sk/download/utildiag/hwi_%s.zip", getVersionNoPoints)
		savename = fmt.Sprintf("hwi_%s.zip", getVersionNoPoints)

		// 检查是否需要下载
		if needDownload(name, getVersion, versionFile, savename, downloadDir) {
			downloadPath := filepath.Join(downloadDir, savename)
			
			// 使用通用下载函数
			err := downloadFile(dlURL, downloadPath)
			if err != nil {
				fmt.Printf("%s下载失败: %v\n", name, err)
			} else {
				// 更新版本信息
				updateVersionFile(name, getVersion, versionFile)
				fmt.Printf("%s下载完成\n", name)
			}
		} else {
			fmt.Printf("%s已是最新版本\n", name)
		}
	} else {
		fmt.Println("无法提取HWINFO版本号")
	}

	// 下载7-ZIP
	name = "7-ZIP"
	fmt.Println("开始下载", name)

	// 获取7-ZIP的HTML内容以提取版本号
	resp, err = http.Get("https://7-zip.org/")
	if err != nil {
		fmt.Println("Error fetching 7-ZIP page:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading 7-ZIP response:", err)
		return
	}
	bodyString = string(bodyBytes)

	// 提取版本号
	re = regexp.MustCompile(`Download 7-Zip (\d+\.\d+)`)
	matches = re.FindStringSubmatch(bodyString)

	if len(matches) > 1 {
		getVersion = matches[1]
		fmt.Printf("找到7-ZIP版本: %s\n", getVersion)

		// 移除版本号中的点
		getVersionNoPoints := strings.ReplaceAll(getVersion, ".", "")
		fmt.Printf("处理后的7-ZIP版本号: %s\n", getVersionNoPoints)

		dlURL = fmt.Sprintf("https://7-zip.org/a/7z%s-x64.exe", getVersionNoPoints)
		savename = filepath.Base(dlURL)

		// 检查是否需要下载
		if needDownload(name, getVersion, versionFile, savename, downloadDir) {
			downloadPath := filepath.Join(downloadDir, savename)
			
			// 使用通用下载函数
			err := downloadFile(dlURL, downloadPath)
			if err != nil {
				fmt.Printf("%s下载失败: %v\n", name, err)
			} else {
				// 更新版本信息
				updateVersionFile(name, getVersion, versionFile)
				fmt.Printf("%s下载完成\n", name)
			}
		} else {
			fmt.Printf("%s已是最新版本\n", name)
		}
	} else {
		fmt.Println("无法提取7-ZIP版本号")
	}

	// 下载AIDA64Extreme
	name = "AIDA64Extreme"
	fmt.Println("开始下载", name)

	// 获取AIDA64Extreme的HTML内容以提取版本号
	resp, err = http.Get("https://www.aida64.com/downloads")
	if err != nil {
		fmt.Println("Error fetching AIDA64Extreme page:", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, err = io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading AIDA64Extreme response:", err)
		return
	}
	bodyString = string(bodyBytes)

	// 提取版本号
	re = regexp.MustCompile(`<td class="version">(\d+\.\d+)`)
	matches = re.FindStringSubmatch(bodyString)

	if len(matches) > 1 {
		getVersion = matches[1]
		fmt.Printf("找到AIDA64Extreme版本: %s\n", getVersion)

		// 移除版本号中的点
		getVersionNoPoints := strings.ReplaceAll(getVersion, ".", "")
		fmt.Printf("处理后的AIDA64Extreme版本号: %s\n", getVersionNoPoints)

		dlURL = fmt.Sprintf("https://download2.aida64.com/aida64extreme%s.zip", getVersionNoPoints)
		savename = filepath.Base(dlURL)

		// 检查是否需要下载
		if needDownload(name, getVersion, versionFile, savename, downloadDir) {
			downloadPath := filepath.Join(downloadDir, savename)
			
			// 使用通用下载函数
			err := downloadFile(dlURL, downloadPath)
			if err != nil {
				fmt.Printf("%s下载失败: %v\n", name, err)
			} else {
				// 更新版本信息
				updateVersionFile(name, getVersion, versionFile)
				fmt.Printf("%s下载完成\n", name)
			}
		} else {
			fmt.Printf("%s已是最新版本\n", name)
		}
	} else {
		fmt.Println("无法提取AIDA64Extreme版本号")
	}

	fmt.Println("所有下载任务已完成")
}