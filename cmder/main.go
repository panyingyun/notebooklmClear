package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ImageInfo represents information about an image in the PDF
type ImageInfo struct {
	PageNr      int
	ObjectNr    int
	GenNr       int
	Name        string
	Width       int
	Height      int
	ColorSpace  string
	BitsPerComp int
	Filter      string
	Size        int64
}

// extractAllImages extracts all images from PDF and prints their information
func extractAllImages(ctx *model.Context) error {
	// Get page count
	pageCount := ctx.XRefTable.PageCount
	if pageCount == 0 {
		return fmt.Errorf("PDF has no pages")
	}

	fmt.Printf("PDF 总页数: %d\n", pageCount)
	fmt.Println("=" + strings.Repeat("=", 80))

	var totalImages int

	// Iterate through all pages
	for pageNr := 1; pageNr <= pageCount; pageNr++ {
		images, err := extractImagesFromPage(ctx, pageNr)
		if err != nil {
			return fmt.Errorf("failed to extract images from page %d: %w", pageNr, err)
		}

		if len(images) > 0 {
			fmt.Printf("\n页面 %d 找到 %d 张图片:\n", pageNr, len(images))
			fmt.Println("-" + strings.Repeat("-", 80))
			for i, img := range images {
				printImageInfo(i+1, img)
			}
			totalImages += len(images)
		}
	}

	fmt.Println("\n" + "=" + strings.Repeat("=", 80))
	fmt.Printf("总计找到 %d 张图片\n", totalImages)

	return nil
}

// extractImagesFromPage extracts all images from a specific page
func extractImagesFromPage(ctx *model.Context, pageNr int) ([]ImageInfo, error) {
	var images []ImageInfo

	// Get page dict
	pageDict, _, _, err := ctx.XRefTable.PageDict(pageNr, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get page dict: %w", err)
	}

	// Get Resources dictionary
	resources, found := pageDict.Find("Resources")
	if !found {
		// Page might inherit resources from parent
		return images, nil
	}

	resDict, ok := resources.(types.Dict)
	if !ok {
		// Might be an indirect reference
		ir, ok := resources.(types.IndirectRef)
		if ok {
			obj, err := ctx.XRefTable.Dereference(ir)
			if err == nil {
				resDict, ok = obj.(types.Dict)
				if !ok {
					return images, nil
				}
			} else {
				return images, nil
			}
		} else {
			return images, nil
		}
	}

	// Get XObject dictionary
	xObjectDict, found := resDict.Find("XObject")
	if !found {
		return images, nil
	}

	xObject, ok := xObjectDict.(types.Dict)
	if !ok {
		// Might be an indirect reference
		ir, ok := xObjectDict.(types.IndirectRef)
		if ok {
			obj, err := ctx.XRefTable.Dereference(ir)
			if err == nil {
				xObject, ok = obj.(types.Dict)
				if !ok {
					return images, nil
				}
			} else {
				return images, nil
			}
		} else {
			return images, nil
		}
	}

	// Iterate through all XObjects
	for name, obj := range xObject {
		ir, ok := obj.(types.IndirectRef)
		if !ok {
			continue
		}

		// Dereference the XObject
		xObjDict, err := ctx.XRefTable.DereferenceXObjectDict(ir)
		if err != nil {
			continue
		}

		// Check if it's an image
		if !xObjDict.Image() {
			continue
		}

		// Extract image information
		imgInfo := extractImageInfo(ctx, pageNr, name, ir, xObjDict)
		images = append(images, imgInfo)
	}

	return images, nil
}

// extractImageInfo extracts information from an image stream dictionary
func extractImageInfo(ctx *model.Context, pageNr int, name string, ir types.IndirectRef, sd *types.StreamDict) ImageInfo {
	imgInfo := ImageInfo{
		PageNr:   pageNr,
		ObjectNr: ir.ObjectNumber.Value(),
		GenNr:    ir.GenerationNumber.Value(),
		Name:     name,
		Size:     int64(len(sd.Raw)),
	}

	// Get Width
	if width := sd.IntEntry("Width"); width != nil {
		imgInfo.Width = *width
	}

	// Get Height
	if height := sd.IntEntry("Height"); height != nil {
		imgInfo.Height = *height
	}

	// Get ColorSpace
	if cs := sd.NameEntry("ColorSpace"); cs != nil {
		imgInfo.ColorSpace = *cs
	} else if csArray := sd.ArrayEntry("ColorSpace"); len(csArray) > 0 {
		if csName, ok := csArray[0].(types.Name); ok {
			imgInfo.ColorSpace = csName.String()
		}
	}

	// Get BitsPerComponent
	if bpc := sd.IntEntry("BitsPerComponent"); bpc != nil {
		imgInfo.BitsPerComp = *bpc
	}

	// Get Filter (compression method)
	if len(sd.FilterPipeline) > 0 {
		var filters []string
		for _, filter := range sd.FilterPipeline {
			filters = append(filters, filter.Name)
		}
		imgInfo.Filter = strings.Join(filters, ", ")
	}

	return imgInfo
}

// printImageInfo prints image information in a formatted way
func printImageInfo(index int, img ImageInfo) {
	fmt.Printf("  图片 #%d:\n", index)
	fmt.Printf("    名称: %s\n", img.Name)
	fmt.Printf("    对象编号: %d %d R\n", img.ObjectNr, img.GenNr)
	fmt.Printf("    尺寸: %d x %d 像素\n", img.Width, img.Height)
	if img.ColorSpace != "" {
		fmt.Printf("    颜色空间: %s\n", img.ColorSpace)
	}
	if img.BitsPerComp > 0 {
		fmt.Printf("    每分量位数: %d\n", img.BitsPerComp)
	}
	if img.Filter != "" {
		fmt.Printf("    压缩方式: %s\n", img.Filter)
	}
	fmt.Printf("    大小: %d 字节\n", img.Size)
	fmt.Println()
}

// extractImagesFromPDF extracts all images from a PDF file and prints their information
func extractImagesFromPDF(inputFile string) error {
	// Read PDF
	ctx, err := api.ReadContextFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read PDF: %w", err)
	}

	// Extract and print all images
	if err := extractAllImages(ctx); err != nil {
		return fmt.Errorf("failed to extract images: %w", err)
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: nbClear <输入PDF文件>")
		fmt.Println("示例: nbClear test/input.pdf")
		fmt.Println("\n功能: 提取并打印 PDF 中所有图片的信息")
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// Check if input file exists
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		log.Fatalf("输入文件不存在: %s", inputFile)
	}

	fmt.Printf("正在分析 PDF: %s\n", inputFile)
	fmt.Println("正在提取所有图片信息...\n")

	// Extract images
	if err := extractImagesFromPDF(inputFile); err != nil {
		log.Fatalf("处理失败: %v", err)
	}
}
