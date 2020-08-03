package main

import (
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
)

func main() {
	router := gin.Default()
	router.LoadHTMLGlob("templates/*")
	router.Static("static", "./static")
	router.GET("/", indexHandler)
	v1 := router.Group("/api/v1")
	{
		v1.GET("/singleFile", HandleSingleFile)
		v1.POST("/upload", HandleUploadFile)
		v1.GET("/download", HandleDownloadFile)
		//PICC不支持delete请求
		//v1.DELETE("/deleteFile", HandleDeleteFile)
		v1.GET("/deleteFile", HandleDeleteFile)
	}
	_ = router.Run("0.0.0.0:8888")
}

func indexHandler(context *gin.Context) {
	context.HTML(http.StatusOK, "index.html", gin.H{"title": "文件上传"})
}

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

// HandleUploadFile 上传单个文件
func HandleUploadFile(context *gin.Context) {
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
func HandleDownloadFile(context *gin.Context) {
	filename := context.Query("filename")
	pwd, _ := os.Getwd()
	context.Writer.WriteHeader(http.StatusOK)
	context.Header("Content-Disposition", "attachment; filename="+filename)
	context.Header("Content-Type", "application/octet-stream")
	context.File(path.Join(pwd, "singleFile", filename))
}

func HandleDeleteFile(context *gin.Context) {
	filename := context.Query("filename")
	pwd, _ := os.Getwd()
	err := os.Remove(path.Join(pwd, "singleFile", filename))
	if err != nil {
		log.Println(err)
		context.JSON(http.StatusInternalServerError, "服务器删除时发生错误")
		return
	}
	context.JSON(http.StatusOK, "服务器文件删除成功")
}

/*
SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64
go build main.go
*/
