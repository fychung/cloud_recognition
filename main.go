package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Readfile(path string) []string {
	f, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return strings.Split(strings.TrimSpace(string(f)), "\n")
}

// 获取波长索引
func GetWaveIndex(cali []string) [2]int {
	var waveIndex [2]int
	var doubleWave [2048]float64
	for i, value := range cali {
		var err error
		doubleWave[i], err = strconv.ParseFloat(strings.TrimSpace(value), 64)
		PrintlnErr(err, value)
	}
	// 计算330和390Channel
	for i, v := range doubleWave {
		if v-330 >= 0 {
			waveIndex[0] = i
			break
		}
	}
	for i, v := range doubleWave {
		if v-390 >= 0 {
			waveIndex[1] = i
			break
		}
	}
	return waveIndex
}

type SpectrumData struct {
	Signal_330 float64
	Signal_390 float64
	Time       float64
	CI         float64
}

// 获取所有光谱数据
func GetSpectrumData(std []string, waveIndex [2]int) SpectrumData {
	signal_330, err := strconv.ParseFloat(strings.TrimSpace(std[waveIndex[0]+3]), 64)
	PrintlnErr(err, std[waveIndex[0]+3])
	signal_390, err1 := strconv.ParseFloat(strings.TrimSpace(std[waveIndex[1]+3]), 64)
	PrintlnErr(err1, std[waveIndex[1]+3])
	dt := strings.Split(strings.TrimSpace(std[2056]), ":")
	h, _ := strconv.ParseFloat(dt[0], 64)
	m, _ := strconv.ParseFloat(dt[1], 64)
	s, _ := strconv.ParseFloat(dt[2], 64)
	second := h*3600 + m*60 + s
	return SpectrumData{signal_330, signal_390, second, signal_330 / signal_390}
}

type PitchData struct {
	FileName    string
	PitchAngle  int
	SpectrumNum int
	SpectrumData
	Tsi float64
}

// 获取所有俯仰角测量数据，去掉水平测量
func GetPitchData(path string, wave [2]int) []*PitchData {
	allfile, err := ioutil.ReadDir(path)
	PrintlnErr(err, path)
	var allPitchData []*PitchData
	for _, file := range allfile {
		name := strings.Split(file.Name(), "_")
		if name[2] == "pitch" {
			spectrumData := GetSpectrumData(Readfile(filepath.Join(path, file.Name())), wave)
			pd := new(PitchData)
			pd.FileName = file.Name()
			pd.PitchAngle, _ = strconv.Atoi(strings.Split(name[3], ".")[0])
			pd.SpectrumNum, _ = strconv.Atoi(name[1])
			pd.CI = spectrumData.CI
			pd.Signal_330 = spectrumData.Signal_330
			pd.Signal_390 = spectrumData.Signal_390
			pd.Time = spectrumData.Time
			allPitchData = append(allPitchData, pd)
		}
	}
	return allPitchData
}

type OneLoopMeasure struct {
	LoopNum  int
	AllPitch []*PitchData
	TSI      []float64
	SP       float64
}

// 计算TSI值方法
func (o *OneLoopMeasure) CalTSI() {
	var tsi []float64
	for i := 2; i < len(o.AllPitch); i++ {
		//t1 := o.AllPitch[i].Time - o.AllPitch[i-1].Time
		//t2 := o.AllPitch[i-1].Time - o.AllPitch[i-2].Time
		//calTsi := ((o.AllPitch[i].CI-o.AllPitch[i-1].CI)/t1 - (o.AllPitch[i-1].CI-o.AllPitch[i-2].CI)/t2) / ((t1 + t2) / 2)
		calTsi := math.Abs((o.AllPitch[i-2].CI+o.AllPitch[i].CI)/2 - o.AllPitch[i-1].CI)
		tsi = append(tsi, calTsi)
		//存回去
		o.AllPitch[i-1].Tsi = calTsi
	}
	o.TSI = tsi
}

func GetAllZenith(allPitchData []*PitchData) []*PitchData {
	var allZenith []*PitchData
	for _, data := range allPitchData {
		if data.PitchAngle == 90 {
			allZenith = append(allZenith, data)
		}
	}
	return allZenith
}

func CalZenithTSI(allZenith []*PitchData) {
	for i := 2; i < len(allZenith); i++ {
		calTsi := math.Abs((allZenith[i-2].CI+allZenith[i].CI)/2 - allZenith[i-1].CI)
		//存回去
		allZenith[i-1].Tsi = calTsi
	}
}

// 计算SP值方法
func (o *OneLoopMeasure) CalSP() {
	maxCI := o.AllPitch[0].CI
	minCI := o.AllPitch[0].CI
	for i := 0; i < len(o.AllPitch); i++ {
		if o.AllPitch[i].CI > maxCI {
			maxCI = o.AllPitch[i].CI
		} else if o.AllPitch[i].CI < minCI {
			minCI = o.AllPitch[i].CI
		}
	}
	o.SP = maxCI - minCI
}

// 获取每一轮循环数据
func GetLoopMeasure(AllPitch []*PitchData, angleNum int) []*OneLoopMeasure {
	var allLoopMeasure []*OneLoopMeasure
	var oneLoopData []*PitchData
	if len(AllPitch)%angleNum != 0 {
		panic("数据条数不是AngleNum的倍数")
	}
	for i := 1; i <= len(AllPitch); i++ {
		oneLoopData = append(oneLoopData, AllPitch[i-1])
		if i%angleNum == 0 {
			olm := new(OneLoopMeasure)
			olm.AllPitch = oneLoopData
			olm.LoopNum = i / angleNum
			olm.CalTSI()
			olm.CalSP()
			allLoopMeasure = append(allLoopMeasure, olm)
			oneLoopData = nil
		}
	}
	return allLoopMeasure
}

// 配置文件参数
type Config struct {
	SpectrumPath string `json:"spectrum_path"`
	CaliPath     string `json:"cali_path"`
	AngleNum     int    `json:"angle_num"`
}

// 全局变量
var config Config

// 读取配置文件
func ReadConfig() (string, string, int) {
	conf, err := ioutil.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	json.Unmarshal(conf, &config)
	return config.SpectrumPath, config.CaliPath, config.AngleNum
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("错误异常:", err)
			fmt.Println("按任意键退出。。。")
			Pause()
			return
		}
	}()

	spec_path, cali_path, angle_num := ReadConfig()
	cali := Readfile(cali_path)
	wave := GetWaveIndex(cali)
	allPitch := GetPitchData(spec_path, wave)
	a := GetLoopMeasure(allPitch, angle_num)
	CalZenithTSI(GetAllZenith(allPitch))
	result, _ := json.MarshalIndent(a, "", "\t")
	ioutil.WriteFile("result.json", result, 0666)
	fmt.Println("完成,按任意键退出。。。")
	Pause()
}

func Pause() {
	b := make([]byte, 1)
	os.Stdin.Read(b)
}
func PrintlnErr(err error, args any) {
	if err != nil {
		fmt.Println(err)
		fmt.Println(args)
	}
}
