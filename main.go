package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"syscall"
	"time"
)

func main() {
	router := gin.Default()
	router.Use(Cors())
	router.LoadHTMLGlob("templates/*")
	router.Static("static", "./static")
	router.GET("/", indexHandler)
	v1 := router.Group("/api/v1")
	{
		v1.GET("/singleFile", HandleSingleFile)
		v1.GET("/listFile", HandleListFile)
		v1.POST("/uploadToCloud", HandleUploadFileToCloud)
		v1.POST("/uploadToServer", HandleUploadFileToServer)
		v1.GET("/downloadFromCloud", HandleDownloadFileFromCloud)
		v1.GET("/getFileList", HandleFileList)
		v1.GET("/downloadFromServer", HandleDownloadFileFromServer)
		//PICC不支持delete请求
		//v1.DELETE("/deleteFile", HandleDeleteFile)
		v1.GET("/deleteFile", HandleDeleteFile)
		v1.GET("/getNetWorkStatus", HandleGetNetWorkStatus)
	}
	_ = router.Run("0.0.0.0:8888")
}

func HandleFileList(context *gin.Context) {
	files, err := ioutil.ReadDir("./upload")
	if err != nil {
		log.Println(err)
	}
	fileList := make(map[string]interface{})
	fileList["code"] = http.StatusOK
	data := make([]map[string]interface{}, 0)
	pwd, _ := os.Getwd()
	for index, f := range files {
		fileMeta := make(map[string]interface{})
		fileInfo, err := os.Stat(path.Join(pwd, "upload", f.Name()))
		if err != nil {
			log.Println(err)
		}
		filename := fileInfo.Name()
		filesize := strconv.Itoa(int(fileInfo.Size()/1024)) + "KB"
		fileSys := fileInfo.Sys().(*syscall.Win32FileAttributeData)
		//fileSys := fileInfo.Sys().(*syscall.Stat_t)
		createTime := fileSys.CreationTime
		//createTime := fileSys.Ctim
		//createTime := fileSys.Ctimespec
		fileMeta["id"] = index + 1
		fileMeta["filename"] = filename
		fileMeta["filesize"] = filesize
		fileMeta["createTime"] = time.Unix(createTime.Nanoseconds()/1e9, 0).Format("2006-01-02 15:04:05")
		//fileMeta["createTime"] = time.Unix(createTime.Sec, createTime.Nsec).Format("2006-01-02 15:04:05")
		data = append(data, fileMeta)
	}
	fileList["data"] = data
	context.JSON(http.StatusOK, fileList)
}

func HandleGetNetWorkStatus(context *gin.Context) {
	encodeUrl := url.QueryEscape("select bytes_recv,bytes_sent from net where time>now()-1h order by time desc limit 2")
	response, err := http.Get("http://127.0.0.1:8086/query?pretty=true&db=telegraf&q=" + encodeUrl)
	result := make(map[string]interface{})
	if err != nil {
		log.Println(err)
	} else {
		defer response.Body.Close()
		res, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Println(err)
		}
		var v interface{}
		_ = json.Unmarshal(res, &v)
		tmp := v.(map[string]interface{})["results"].([]interface{})[0].(map[string]interface{})["series"].([]interface{})[0].(map[string]interface{})["values"].([]interface{})
		result["time"] = time.Now().Format("15:04:05")
		// 下行流量
		result["down"] = (tmp[0].([]interface{})[1].(float64) - tmp[1].([]interface{})[1].(float64)) / 1000
		// 上行流量
		result["up"] = (tmp[0].([]interface{})[2].(float64) - tmp[1].([]interface{})[2].(float64)) / 1000
	}
	context.JSON(http.StatusOK, result)
}

func indexHandler(context *gin.Context) {
	context.HTML(http.StatusOK, "index.html", gin.H{"title": "文件上传"})
}

//检测文件接口
func HandleSingleFile(context *gin.Context) {
	pwd, _ := os.Getwd()
	fullPath := path.Join(pwd, "singleFile")
	var fileInfoList []os.FileInfo
	if checkPath(fullPath) {
		fileInfoList, _ = ioutil.ReadDir(fullPath)
		if len(fileInfoList) > 0 {
			context.JSON(http.StatusOK, fileInfoList[0].Name())
			return
		}
		context.JSON(http.StatusOK, nil)
		return
	}
	context.JSON(http.StatusInternalServerError, nil)
}
func HandleListFile(context *gin.Context) {
	pwd, _ := os.Getwd()
	fullPath := path.Join(pwd, "upload")
	var fileInfoList []os.FileInfo
	if checkPath(fullPath) {
		fileInfoList, _ = ioutil.ReadDir(fullPath)
		if len(fileInfoList) > 0 {
			context.JSON(http.StatusOK, fileInfoList[0].Name())
			return
		}
		context.JSON(http.StatusOK, nil)
		return
	}
	context.JSON(http.StatusInternalServerError, nil)
}
func checkPath(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			_ = os.MkdirAll(path, 0777)
			return true
		} else {
			return false
		}
	}
	return true
}

//外网向云桌面内传文件
func HandleUploadFileToCloud(context *gin.Context) {
	file, header, err := context.Request.FormFile("file")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"msg": "文件上传失败"})
		return
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"msg": "文件读取失败"})
		return
	}
	filename := header.Filename
	pwd, _ := os.Getwd()
	_ = os.MkdirAll(path.Join(pwd, "tmp"), 0777)
	err = ioutil.WriteFile(path.Join(pwd, "tmp", filename), content, 0777)
	if err != nil {
		log.Println(err)
		context.JSON(http.StatusInternalServerError, gin.H{"msg": "文件写入临时目录失败"})
		return
	}
	_ = os.Rename(path.Join(pwd, "tmp", filename), path.Join(pwd, "singleFile", filename))
	context.JSON(http.StatusOK, gin.H{"msg": "上传成功"})
}

//从云桌面取文件到外网服务器
func HandleUploadFileToServer(context *gin.Context) {
	file, header, err := context.Request.FormFile("file")
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"msg": "文件上传失败"})
		return
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"msg": "文件读取失败"})
		return
	}
	filename := header.Filename
	pwd, _ := os.Getwd()
	_ = os.MkdirAll(path.Join(pwd, "upload"), 0777)
	err = ioutil.WriteFile(path.Join(pwd, "upload", filename), content, 0777)
	if err != nil {
		log.Println(err)
		context.JSON(http.StatusInternalServerError, gin.H{"msg": path.Join(pwd, "upload", filename) + "写入失败"})
		return
	}
	context.JSON(http.StatusOK, gin.H{"msg": "上传成功"})
}

//agent下载文件到云桌面
func HandleDownloadFileFromServer(context *gin.Context) {
	filename := context.Query("filename")
	pwd, _ := os.Getwd()
	context.Writer.WriteHeader(http.StatusOK)
	context.Header("Content-Disposition", "attachment; filename="+filename)
	context.Header("Content-Type", "application/octet-stream")
	context.File(path.Join(pwd, "singleFile", filename))
}

//用户下载从云桌面拿到的文件
func HandleDownloadFileFromCloud(context *gin.Context) {
	log.Println(11111)
	filename := context.Query("filename")
	pwd, _ := os.Getwd()
	context.Writer.WriteHeader(http.StatusOK)
	context.Header("Content-Disposition", "attachment; filename="+filename)
	context.Header("Content-Type", "application/octet-stream")
	context.File(path.Join(pwd, "upload", filename))
}

func HandleDeleteFile(context *gin.Context) {
	filename := context.Query("filename")
	filepath := context.Query("filepath")
	pwd, _ := os.Getwd()
	if filepath == "upload" {
		err := os.Remove(path.Join(pwd, filepath, filename))
		if err != nil {
			log.Println(err)
			context.JSON(http.StatusInternalServerError, "服务器删除时发生错误")
			return
		}
	} else {
		err := os.Remove(path.Join(pwd, "singleFile", filename))
		if err != nil {
			log.Println(err)
			context.JSON(http.StatusInternalServerError, "服务器删除时发生错误")
			return
		}
	}
	context.JSON(http.StatusOK, "服务器文件删除成功")
}
func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
		c.Header("Access-Control-Allow-Credentials", "true")

		if method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
		}

		c.Next()
	}
}

/*
SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
go build main.go
*/
