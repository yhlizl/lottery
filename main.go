package main

import (
	"errors"
	"fmt"
	"math/rand"
	"mime/multipart"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

func SetupDB() (*gorm.DB, error) {
	// 修改下面的數據為你的 MySQL 連線信息
	dsn := "root:usbw@tcp(127.0.0.1:3306)/lottery?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// 自動遷移，創建表格
	db.AutoMigrate(&Lottery{}, &Removed{})
	return db, nil
}

type FormData struct {
	Picture *multipart.FileHeader `form:"picture" binding:"required"`
	// 其他表單字段，如果有的話
}

type Lottery struct {
	gorm.Model
	User    string `gorm:"column:user"`
	Date    string `gorm:"column:date"`
	Picture []byte `gorm:"column:picture"`
}
type Removed struct {
	Num int `gorm:"column:num"`
}

func main() {
	r := gin.Default()

	// 使用 CORS 中間件
	r.Use(cors.Default())

	// 初始化 MySQL 連線
	var err error
	db, err = SetupDB()
	if err != nil {
		panic("Failed to connect to database")
	}

	// 添加路由
	r.POST("/lottery", uploadHandler)
	r.POST("/getlottery", getLotteryData)
	r.Run(":8080")
}
func uploadHandler(c *gin.Context) {
	// 檢查是否已經上傳過
	var hasUploaded bool
	if _, err := c.Cookie("hasUploaded"); err == nil {
		hasUploaded = true
	}

	if hasUploaded {
		c.JSON(400, gin.H{"error": "You have already uploaded a file."})
		return
	}

	// 上傳文件
	var formData FormData
	if err := c.ShouldBind(&formData); err != nil {
		c.JSON(400, gin.H{"error": "Error parsing form data", "details": err.Error()})
		return
	}

	// 檢查數據庫中是否已存在相同的圖片
	if isDuplicate, err := isDuplicatePicture(formData.Picture.Filename); err != nil {
		c.JSON(500, gin.H{"error": "Error checking duplicate picture", "details": err.Error()})
		return
	} else if isDuplicate {
		c.JSON(400, gin.H{"error": "Duplicate picture. You cannot upload the same picture again."})
		return
	}

	// 在這裡可以對 lottery 進行數據庫操作，例如插入或更新
	// 生成當前日期
	currentDate := time.Now().Format("2006-01-02")
	lottery := Lottery{
		User: "SomeUser", // 此處應替換為實際用戶
		Date: currentDate,
	}

	// 保存文件
	if err := c.SaveUploadedFile(formData.Picture, "uploads/"+formData.Picture.Filename); err != nil {
		c.JSON(500, gin.H{"error": "Error saving file", "details": err.Error()})
		return
	}

	// 插入數據
	lottery.Picture = []byte(formData.Picture.Filename)
	if err := db.Create(&lottery).Error; err != nil {
		c.JSON(500, gin.H{"error": "Error inserting data into database", "details": err.Error()})
		return
	}

	// 使用本地存儲標記為已經上傳
	c.SetCookie("hasUploaded", "true", 0, "/", "localhost", false, true)

	// 隨機選取不在 removed 表中的數字
	randomNumber := getRandomNumber()

	// 插入到 removed 表中
	if err := db.Table("removed").Create(&Removed{Num: randomNumber}).Error; err != nil {
		// 處理錯誤
		fmt.Println("Error inserting data into removed table:", err)
		c.JSON(500, gin.H{"error": "Error inserting data into removed table", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Lottery uploaded successfully", "result": randomNumber})
}

// isDuplicatePicture 檢查數據庫中是否已存在相同的圖片
func isDuplicatePicture(filename string) (bool, error) {
	var existingLottery Lottery
	result := db.Where("picture = ?", filename).First(&existingLottery)
	if result.RowsAffected > 0 {
		return true, nil
	} else if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, result.Error
	}

	return false, nil
}

func getLotteryData(c *gin.Context) {
	var removedNumbers []int
	var awardNumbers []int

	// 提取 removed 的數據
	if err := db.Table("removed").Pluck("num", &removedNumbers).Error; err != nil {
		c.JSON(500, gin.H{"error": "Error fetching removed data", "details": err.Error()})
		return
	}

	// 提取 award 的數據
	if err := db.Table("award").Pluck("num", &awardNumbers).Error; err != nil {
		c.JSON(500, gin.H{"error": "Error fetching award data", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"removed": removedNumbers, "大獎": awardNumbers})
}
func getRandomNumber() int {
	source := rand.NewSource(time.Now().UnixNano())
	random := rand.New(source)

	// 生成一個介於1和40之間的隨機數字
	randomNumber := random.Intn(40) + 1

	// 檢查生成的數字是否在 removed 表中，如果是，重新生成
	for isNumberInRemoved(randomNumber) {
		randomNumber = random.Intn(40) + 1
	}

	return randomNumber
}

func isNumberInRemoved(number int) bool {
	var count int64
	db.Table("removed").Where("num = ?", number).Count(&count)
	return count > 0
}
