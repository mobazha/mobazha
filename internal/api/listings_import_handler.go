package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/h2non/filetype"
	"github.com/mobazha/mobazha3.0/pkg/contracts"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"github.com/xuri/excelize/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Sheet names
	sheetProducts = "Products"
	sheetVariants = "Variants"

	// Sheet names in Chinese
	sheetProductsZH = "商品列表"
	sheetVariantsZH = "商品变体"
)

// Column mappings for English template
var columnsEN = map[string]int{
	"title":              0,
	"contractType":       1,
	"price":              2,
	"pricingCurrency":    3,
	"description":        4,
	"shortDescription":   5,
	"productType":        6,
	"tags":               7,
	"condition":          8,
	"nsfw":               9,
	"images":             10,
	"introVideo":         11,
	"processingTime":     12,
	"grams":              13,
	"quantity":           14,
	"brand":              15,
	"weightUnit":         16,
	"status":             17,
	"regularPrice":       18,
}

// Column mappings for Chinese template
var columnsZH = map[string]int{
	"商品标题":  0,
	"商品类型":  1,
	"价格":    2,
	"定价货币":  3,
	"详细描述":  4,
	"简短描述":  5,
	"分类":    6,
	"标签":    7,
	"新旧状态":  8,
	"成人内容":  9,
	"图片文件名": 10,
	"介绍视频":  11,
	"处理时间":  12,
	"重量(克)": 13,
	"库存数量":  14,
	"品牌":    15,
	"重量单位":  16,
	"发布状态":  17,
	"原价":    18,
}

// ImportResult represents the result of a batch import operation
type ImportResult struct {
	Total           int            `json:"total"`
	Created         int            `json:"created"`
	Updated         int            `json:"updated"`
	Failed          int            `json:"failed"`
	CreatedItems    []ImportedItem `json:"createdItems"`
	UpdatedItems    []ImportedItem `json:"updatedItems"`
	Errors          []ImportError  `json:"errors"`
	ProfileImported bool           `json:"profileImported,omitempty"`
	ProfileError    string         `json:"profileError,omitempty"`
}

// ImportedItem represents a successfully imported item
type ImportedItem struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

// ImportError represents an error during import
type ImportError struct {
	Row   int    `json:"row"`
	Title string `json:"title"`
	Error string `json:"error"`
}

// handleGETListingsTemplate generates and returns an Excel template for batch import
func (g *Gateway) handleGETListingsTemplate(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang != "zh" {
		lang = "en"
	}

	f := excelize.NewFile()
	defer f.Close()

	// Create Products sheet
	productsSheet := sheetProducts
	variantsSheet := sheetVariants
	if lang == "zh" {
		productsSheet = sheetProductsZH
		variantsSheet = sheetVariantsZH
	}

	// Rename default sheet to Products
	f.SetSheetName("Sheet1", productsSheet)

	// Create Variants sheet
	f.NewSheet(variantsSheet)

	// Set up Products sheet headers
	if lang == "en" {
		headers := []string{
			"title", "contractType", "price", "pricingCurrency", "description",
			"shortDescription", "productType", "tags", "condition", "nsfw",
			"images", "introVideo", "processingTime", "grams",
			"quantity", "brand", "weightUnit", "status", "regularPrice",
		}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(productsSheet, cell, h)
		}

		// Add example row
		exampleRow := []interface{}{
			"Example Product", "PHYSICAL_GOOD", "99.99", "USD",
			"Product description here", "Short description",
			"Electronics,Gadgets", "new,popular", "New", "false",
			"image1.jpg,image2.png", "intro.mp4", "1-3 days", "500",
			"100", "BrandName", "g", "published", "129.99",
		}
		for i, v := range exampleRow {
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			f.SetCellValue(productsSheet, cell, v)
		}

		// Variants sheet headers - selections format: "Option1:Value1,Option2:Value2"
		variantHeaders := []string{"productTitle", "selections", "price", "quantity", "productID", "barcode"}
		for i, h := range variantHeaders {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(variantsSheet, cell, h)
		}

		// Add variant examples - Color x Size combinations
		variantExamples := [][]interface{}{
			{"Example Product", "Color:Red,Size:S", "2999", "50", "SKU-RED-S", ""},
			{"Example Product", "Color:Red,Size:M", "2999", "60", "SKU-RED-M", ""},
			{"Example Product", "Color:Red,Size:L", "3499", "40", "SKU-RED-L", ""},
			{"Example Product", "Color:Blue,Size:S", "0", "30", "SKU-BLUE-S", ""},
			{"Example Product", "Color:Blue,Size:M", "0", "80", "SKU-BLUE-M", ""},
			{"Example Product", "Color:Blue,Size:L", "10", "25", "SKU-BLUE-L", ""},
		}
		for rowIdx, example := range variantExamples {
			for colIdx, v := range example {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
				f.SetCellValue(variantsSheet, cell, v)
			}
		}
	} else {
		// Chinese headers
		headers := []string{
			"商品标题", "商品类型", "价格", "定价货币", "详细描述",
			"简短描述", "分类", "标签", "新旧状态", "成人内容",
			"图片文件名", "介绍视频", "处理时间", "重量(克)",
			"库存数量", "品牌", "重量单位", "发布状态", "原价",
		}
		for i, h := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(productsSheet, cell, h)
		}

		// Add example row
		exampleRow := []interface{}{
			"示例商品", "PHYSICAL_GOOD", "99.99", "CNY",
			"商品详细描述", "简短描述",
			"电子产品,数码", "新品,热门", "New", "false",
			"image1.jpg,image2.png", "intro.mp4", "1-3天", "500",
			"100", "品牌名", "g", "published", "129.99",
		}
		for i, v := range exampleRow {
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			f.SetCellValue(productsSheet, cell, v)
		}

		// Variants sheet headers - selections format: "选项1:值1,选项2:值2"
		variantHeaders := []string{"关联商品标题", "选项组合", "价格", "库存数量", "SKU编号", "条码"}
		for i, h := range variantHeaders {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue(variantsSheet, cell, h)
		}

		// Add variant examples - 颜色 x 尺码 组合
		variantExamples := [][]interface{}{
			{"示例商品", "颜色:红色,尺码:S", "0", "50", "SKU-RED-S", ""},
			{"示例商品", "颜色:红色,尺码:M", "0", "60", "SKU-RED-M", ""},
			{"示例商品", "颜色:红色,尺码:L", "10", "40", "SKU-RED-L", ""},
			{"示例商品", "颜色:蓝色,尺码:S", "0", "30", "SKU-BLUE-S", ""},
			{"示例商品", "颜色:蓝色,尺码:M", "0", "80", "SKU-BLUE-M", ""},
			{"示例商品", "颜色:蓝色,尺码:L", "10", "25", "SKU-BLUE-L", ""},
		}
		for rowIdx, example := range variantExamples {
			for colIdx, v := range example {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
				f.SetCellValue(variantsSheet, cell, v)
			}
		}
	}

	// Set column widths for better readability
	f.SetColWidth(productsSheet, "A", "S", 18)
	f.SetColWidth(variantsSheet, "A", "F", 18)

	// Create header style
	style, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})
	f.SetCellStyle(productsSheet, "A1", "S1", style)
	f.SetCellStyle(variantsSheet, "A1", "F1", style)

	// Write to response
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	filename := "listings_template_" + lang + ".xlsx"
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)

	if err := f.Write(w); err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
}

var profileImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// isProfileImage returns true if the file is a root-level avatar.* or header.* image
// (not inside images/ or videos/ subdirectories).
func isProfileImage(normalizedName, basename string) (slot string, ok bool) {
	if strings.Contains(normalizedName, "/images/") || strings.HasPrefix(normalizedName, "images/") ||
		strings.Contains(normalizedName, "/videos/") || strings.HasPrefix(normalizedName, "videos/") {
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(basename))
	if !profileImageExts[ext] {
		return "", false
	}
	nameNoExt := strings.TrimSuffix(strings.ToLower(basename), ext)
	switch nameNoExt {
	case "avatar":
		return "avatar", true
	case "header":
		return "header", true
	}
	return "", false
}

// handlePOSTListingsImport handles the batch import of listings from a ZIP file.
// It auto-detects the data format inside the ZIP:
//   - If a .json file is found → JSON import (supports shippingProfiles + collections)
//   - If a .xlsx file is found → Excel import
func (g *Gateway) handlePOSTListingsImport(w http.ResponseWriter, r *http.Request) {
	maxZipSize := g.nodeManager.GetMaxImportZipSize()
	maxVideoSize := g.nodeManager.GetMaxImportVideoSize()

	r.Body = http.MaxBytesReader(w, r.Body, maxZipSize)

	if err := r.ParseMultipartForm(maxZipSize); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("file too large or invalid: %s", err.Error()))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading file: %s", err.Error()))
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		ErrorResponse(w, http.StatusBadRequest, "file must be a ZIP archive")
		return
	}

	zipData, err := io.ReadAll(file)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading file: %s", err.Error()))
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ZIP file: %s", err.Error()))
		return
	}

	var excelFile *zip.File
	var jsonFile *zip.File
	var avatarData, headerData []byte
	images := make(map[string][]byte)
	videos := make(map[string][]byte)

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX") {
			continue
		}

		normalizedName := strings.TrimPrefix(strings.ToLower(f.Name), "./")
		filename := filepath.Base(f.Name)

		if slot, ok := isProfileImage(normalizedName, filename); ok {
			if data, err := readZipFile(f); err == nil {
				if slot == "avatar" {
					avatarData = data
				} else {
					headerData = data
				}
			}
			continue
		}

		switch {
		case strings.HasSuffix(normalizedName, ".json"):
			if jsonFile == nil {
				jsonFile = f
			}
		case strings.HasSuffix(normalizedName, ".xlsx"):
			excelFile = f
		case strings.HasPrefix(normalizedName, "images/") || strings.Contains(normalizedName, "/images/"):
			if data, err := readZipFile(f); err == nil {
				images[filename] = data
			}
		case strings.HasPrefix(normalizedName, "videos/") || strings.Contains(normalizedName, "/videos/"):
			if data, err := readZipFile(f); err == nil && int64(len(data)) <= maxVideoSize {
				videos[filename] = data
			}
		}
	}

	// JSON format takes priority over Excel
	if jsonFile != nil {
		g.importFromJSON(w, r, jsonFile, images, videos, avatarData, headerData)
		return
	}

	if excelFile == nil {
		ErrorResponse(w, http.StatusBadRequest, "no Excel (.xlsx) or JSON (.json) file found in ZIP archive")
		return
	}

	// Excel format processing
	excelData, err := readZipFile(excelFile)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading Excel file: %s", err.Error()))
		return
	}

	xlsx, err := excelize.OpenReader(bytes.NewReader(excelData))
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error parsing Excel file: %s", err.Error()))
		return
	}
	defer xlsx.Close()

	lang := "en"
	sheets := xlsx.GetSheetList()
	for _, sheet := range sheets {
		if sheet == sheetProductsZH || sheet == sheetVariantsZH {
			lang = "zh"
			break
		}
	}

	listingSvc := getListingService(r)
	mediaSvc := getMediaService(r)

	result, err := g.processListingsImport(listingSvc, mediaSvc, xlsx, lang, images, videos)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	sanitizedJSONResponse(w, result)
}

// processListingsImport processes the Excel data and creates/updates listings
func (g *Gateway) processListingsImport(listingSvc contracts.ListingService, mediaSvc contracts.MediaService, xlsx *excelize.File, lang string, images, videos map[string][]byte) (*ImportResult, error) {
	result := &ImportResult{
		CreatedItems: []ImportedItem{},
		UpdatedItems: []ImportedItem{},
		Errors:       []ImportError{},
	}

	// Determine sheet names based on language
	productsSheet := sheetProducts
	variantsSheet := sheetVariants
	if lang == "zh" {
		productsSheet = sheetProductsZH
		variantsSheet = sheetVariantsZH
	}

	// Read products
	rows, err := xlsx.GetRows(productsSheet)
	if err != nil {
		return nil, fmt.Errorf("error reading products sheet: %w", err)
	}

	if len(rows) < 2 {
		return result, nil // No data rows
	}

	// Parse header to get column indices
	columns := g.parseHeaders(rows[0], lang)

	// Read variants
	variants := g.readVariants(xlsx, variantsSheet, lang)

	// Get existing listings for duplicate detection
	existingListings, _ := listingSvc.GetMyListings()
	existingByTitle := make(map[string]string) // title -> slug
	for _, listing := range existingListings {
		existingByTitle[listing.Title] = listing.Slug
	}

	// Process each row
	for i, row := range rows[1:] {
		rowNum := i + 2 // Excel row number (1-based, header is row 1)

		if len(row) == 0 {
			continue
		}

		listing, err := g.parseListingRow(row, columns, lang)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: g.getCellValue(row, columns["title"]),
				Error: err.Error(),
			})
			result.Failed++
			continue
		}

		// Process images
		if imgErr := g.processListingImages(mediaSvc, listing, row, columns, images); imgErr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: imgErr.Error(),
			})
			result.Failed++
			continue
		}

		// Process intro video
		if vidErr := g.processListingVideo(mediaSvc, listing, row, columns, videos); vidErr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: vidErr.Error(),
			})
			result.Failed++
			continue
		}

		// Process variants
		g.processListingVariants(listing, listing.Item.Title, variants)

		// For non-variant items, set quantity via a default SKU if provided
		quantityStr := g.getCellValue(row, columns["quantity"])
		if quantityStr != "" && len(listing.Item.Skus) == 0 {
			listing.Item.Skus = append(listing.Item.Skus, &pb.Listing_Item_Sku{
				Quantity: quantityStr,
			})
		}

		// Validate: at least one image is required
		if len(listing.Item.Images) == 0 {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: "at least one image is required",
			})
			result.Failed++
			continue
		}

		// Check if listing exists (by title)
		isUpdate := false
		if existingSlug, exists := existingByTitle[listing.Item.Title]; exists {
			listing.Slug = existingSlug
			isUpdate = true
		}

		// Save listing
		saveErr := listingSvc.SaveListing(listing, nil)

		if saveErr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: saveErr.Error(),
			})
			result.Failed++
			continue
		}

		if isUpdate {
			result.Updated++
			result.UpdatedItems = append(result.UpdatedItems, ImportedItem{
				Slug:  listing.Slug,
				Title: listing.Item.Title,
			})
		} else {
			result.Created++
			result.CreatedItems = append(result.CreatedItems, ImportedItem{
				Slug:  listing.Slug,
				Title: listing.Item.Title,
			})
			// Add to existing map for subsequent duplicate detection
			existingByTitle[listing.Item.Title] = listing.Slug
		}

		result.Total++
	}

	return result, nil
}

// parseHeaders parses the header row to get column indices
func (g *Gateway) parseHeaders(headerRow []string, lang string) map[string]int {
	result := make(map[string]int)

	var expectedColumns map[string]int
	if lang == "zh" {
		expectedColumns = columnsZH
	} else {
		expectedColumns = columnsEN
	}

	// Map header names to indices
	for i, header := range headerRow {
		header = strings.TrimSpace(header)
		for name, _ := range expectedColumns {
			if header == name {
				result[g.normalizeColumnName(name, lang)] = i
				break
			}
		}
	}

	return result
}

// normalizeColumnName converts Chinese column names to English keys
func (g *Gateway) normalizeColumnName(name string, lang string) string {
	if lang == "en" {
		return name
	}

	// Chinese to English mapping
	mapping := map[string]string{
		"商品标题":  "title",
		"商品类型":  "contractType",
		"价格":    "price",
		"定价货币":  "pricingCurrency",
		"详细描述":  "description",
		"简短描述":  "shortDescription",
		"分类":    "productType",
		"标签":    "tags",
		"新旧状态":  "condition",
		"成人内容":  "nsfw",
		"图片文件名": "images",
		"介绍视频":  "introVideo",
		"处理时间":  "processingTime",
		"重量(克)": "grams",
		"库存数量":  "quantity",
		"品牌":    "brand",
		"重量单位":  "weightUnit",
		"发布状态":  "status",
		"原价":    "regularPrice",
	}

	if en, ok := mapping[name]; ok {
		return en
	}
	return name
}

// parseListingRow parses a single row into a Listing object
func (g *Gateway) parseListingRow(row []string, columns map[string]int, lang string) (*pb.Listing, error) {
	title := g.getCellValue(row, columns["title"])
	if title == "" {
		return nil, errors.New("title is required")
	}

	contractTypeStr := g.getCellValue(row, columns["contractType"])
	contractType := g.parseContractType(contractTypeStr)

	priceStr := g.getCellValue(row, columns["price"])
	if priceStr == "" {
		return nil, errors.New("price is required")
	}

	currency := g.getCellValue(row, columns["pricingCurrency"])
	if currency == "" {
		return nil, errors.New("pricingCurrency is required")
	}

	// Convert price to integer format (e.g., "99.99" -> "9999" for divisibility=2)
	priceInt, err := g.convertPriceToInt(priceStr, 2) // Default divisibility is 2
	if err != nil {
		return nil, fmt.Errorf("invalid price format: %w", err)
	}

	// Parse optional fields
	nsfw := strings.ToLower(g.getCellValue(row, columns["nsfw"])) == "true"

	gramsStr := g.getCellValue(row, columns["grams"])
	var grams uint32
	if gramsStr != "" {
		if g, err := strconv.ParseUint(gramsStr, 10, 32); err == nil {
			grams = uint32(g)
		}
	}

	// Parse productType and tags
	productType := strings.TrimSpace(g.getCellValue(row, columns["productType"]))

	tagsStr := g.getCellValue(row, columns["tags"])
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
	}

	// Parse optional new fields
	brand := strings.TrimSpace(g.getCellValue(row, columns["brand"]))
	weightUnit := strings.TrimSpace(g.getCellValue(row, columns["weightUnit"]))
	status := strings.TrimSpace(g.getCellValue(row, columns["status"]))
	if status == "" {
		status = "published"
	}

	var regularPriceInt string
	regularPriceStr := g.getCellValue(row, columns["regularPrice"])
	if regularPriceStr != "" {
		rp, rpErr := g.convertPriceToInt(regularPriceStr, 2)
		if rpErr == nil {
			regularPriceInt = rp
		}
	}

	listing := &pb.Listing{
		Metadata: &pb.Listing_Metadata{
			ContractType: contractType,
			Format:       pb.Listing_Metadata_FIXED_PRICE,
			PricingCurrency: &pb.Currency{
				Code:         strings.ToUpper(currency),
				Divisibility: 2,
			},
			Expiry: timestamppb.New(time.Date(2037, 12, 31, 0, 0, 0, 0, time.UTC)),
		},
		Status: status,
		Item: &pb.Listing_Item{
			Title:            title,
			Description:      g.getCellValue(row, columns["description"]),
			ShortDescription: g.getCellValue(row, columns["shortDescription"]),
			Price:            priceInt,
			RegularPrice:     regularPriceInt,
			Condition:        g.getCellValue(row, columns["condition"]),
			ProcessingTime:   g.getCellValue(row, columns["processingTime"]),
			Nsfw:             nsfw,
			ProductType:      productType,
			Tags:             tags,
			Grams:            grams,
			Brand:            brand,
			WeightUnit:       weightUnit,
		},
	}

	return listing, nil
}

// parseContractType converts string to ContractType enum
func (g *Gateway) parseContractType(s string) pb.Listing_Metadata_ContractType {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "PHYSICAL_GOOD":
		return pb.Listing_Metadata_PHYSICAL_GOOD
	case "DIGITAL_GOOD":
		return pb.Listing_Metadata_DIGITAL_GOOD
	case "SERVICE":
		return pb.Listing_Metadata_SERVICE
	case "CLASSIFIED":
		return pb.Listing_Metadata_CLASSIFIED
	case "CRYPTOCURRENCY":
		return pb.Listing_Metadata_CRYPTOCURRENCY
	case "RWA_TOKEN":
		return pb.Listing_Metadata_RWA_TOKEN
	default:
		return pb.Listing_Metadata_PHYSICAL_GOOD
	}
}

// getCellValue safely gets a cell value from a row
func (g *Gateway) getCellValue(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

// processListingImages processes and uploads images for a listing
func (g *Gateway) processListingImages(mediaSvc contracts.MediaService, listing *pb.Listing, row []string, columns map[string]int, images map[string][]byte) error {
	imagesStr := g.getCellValue(row, columns["images"])
	if imagesStr == "" {
		return nil
	}

	imageNames := strings.Split(imagesStr, ",")
	for _, imgName := range imageNames {
		imgName = strings.TrimSpace(imgName)
		if imgName == "" {
			continue
		}

		imgData, ok := images[imgName]
		if !ok {
			// Try to find with case-insensitive match
			for name, data := range images {
				if strings.EqualFold(name, imgName) {
					imgData = data
					ok = true
					break
				}
			}
		}

		if !ok {
			continue // Skip missing images
		}

		// Check if it's a valid image
		if !filetype.IsImage(imgData) {
			continue
		}

		// Upload image with variants
		result, err := mediaSvc.UploadMedia(context.Background(), imgData, imgName, contracts.UploadOpts{Variants: true})
		if err != nil {
			return fmt.Errorf("failed to upload image %s: %w", imgName, err)
		}

		if result.Hashes != nil {
			listing.Item.Images = append(listing.Item.Images, &pb.Image{
				Filename: imgName,
				Original: result.Hashes.Original,
				Large:    result.Hashes.Large,
				Medium:   result.Hashes.Medium,
				Small:    result.Hashes.Small,
				Tiny:     result.Hashes.Tiny,
			})
		}
	}

	return nil
}

// processListingVideo processes and uploads intro video for a listing
func (g *Gateway) processListingVideo(mediaSvc contracts.MediaService, listing *pb.Listing, row []string, columns map[string]int, videos map[string][]byte) error {
	videoName := g.getCellValue(row, columns["introVideo"])
	if videoName == "" {
		return nil
	}

	videoData, ok := videos[videoName]
	if !ok {
		// Try case-insensitive match
		for name, data := range videos {
			if strings.EqualFold(name, videoName) {
				videoData = data
				ok = true
				break
			}
		}
	}

	if !ok {
		return nil // Skip missing video
	}

	// Check video size
	maxVideoSize := g.nodeManager.GetMaxImportVideoSize()
	if int64(len(videoData)) > maxVideoSize {
		return fmt.Errorf("video %s exceeds maximum size of %dMB", videoName, maxVideoSize/(1<<20))
	}

	// Check if it's a valid video
	if !filetype.IsVideo(videoData) {
		return fmt.Errorf("file %s is not a valid video", videoName)
	}

	// Upload video
	result, err := mediaSvc.UploadMedia(context.Background(), videoData, videoName, contracts.UploadOpts{MaxBytes: maxVideoSize})
	if err != nil {
		return fmt.Errorf("failed to upload video %s: %w", videoName, err)
	}

	listing.Item.IntroVideo = &pb.File{
		Filename: videoName,
		Hash:     result.Hash,
	}

	return nil
}

// Selection represents a single option:value pair
type Selection struct {
	Option  string
	Variant string
}

// VariantData holds parsed SKU information with option combinations
type VariantData struct {
	ProductTitle string
	Selections   []Selection // e.g., [{Option: "Color", Variant: "Red"}, {Option: "Size", Variant: "S"}]
	Price        string      // 变体绝对价格（Shopify 风格）
	Quantity     string
	ProductID    string
	Barcode      string
}

// parseSelections parses "Color:Red,Size:S" format into []Selection
func parseSelections(s string) []Selection {
	var selections []Selection
	if s == "" {
		return selections
	}

	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			selections = append(selections, Selection{
				Option:  strings.TrimSpace(parts[0]),
				Variant: strings.TrimSpace(parts[1]),
			})
		}
	}
	return selections
}

// readVariants reads all variants from the variants sheet
func (g *Gateway) readVariants(xlsx *excelize.File, sheetName string, lang string) []VariantData {
	var variants []VariantData

	rows, err := xlsx.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		return variants
	}

	// Parse headers
	columns := make(map[string]int)
	for i, header := range rows[0] {
		header = strings.TrimSpace(header)
		switch header {
		case "productTitle", "关联商品标题":
			columns["productTitle"] = i
		case "selections", "选项组合":
			columns["selections"] = i
		case "price", "价格":
			columns["price"] = i
		case "quantity", "库存数量":
			columns["quantity"] = i
		case "productID", "SKU编号":
			columns["productID"] = i
		case "barcode", "条码":
			columns["barcode"] = i
		}
	}

	// Parse data rows
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}

		selectionsStr := g.getCellValue(row, columns["selections"])
		selections := parseSelections(selectionsStr)

		variant := VariantData{
			ProductTitle: g.getCellValue(row, columns["productTitle"]),
			Selections:   selections,
			Price:        g.getCellValue(row, columns["price"]),
			Quantity:     g.getCellValue(row, columns["quantity"]),
			ProductID:    g.getCellValue(row, columns["productID"]),
			Barcode:      g.getCellValue(row, columns["barcode"]),
		}

		if variant.ProductTitle != "" && len(variant.Selections) > 0 {
			variants = append(variants, variant)
		}
	}

	return variants
}

// processListingVariants adds variants to a listing
func (g *Gateway) processListingVariants(listing *pb.Listing, productTitle string, variants []VariantData) {
	// Collect all options and their variants from SKU data
	// optionVariants: map[optionName]map[variantName]bool
	optionVariants := make(map[string]map[string]bool)

	for _, v := range variants {
		if v.ProductTitle != productTitle {
			continue
		}

		for _, sel := range v.Selections {
			if optionVariants[sel.Option] == nil {
				optionVariants[sel.Option] = make(map[string]bool)
			}
			optionVariants[sel.Option][sel.Variant] = true
		}
	}

	// Create options with their variants
	for optName, variantSet := range optionVariants {
		option := &pb.Listing_Item_Option{
			Name: optName,
		}

		for variantName := range variantSet {
			option.Variants = append(option.Variants, &pb.Listing_Item_Option_Variant{
				Name: variantName,
			})
		}

		listing.Item.Options = append(listing.Item.Options, option)
	}

	// Create SKUs from variants
	for _, v := range variants {
		if v.ProductTitle != productTitle {
			continue
		}

		var selections []*pb.Listing_Item_Sku_Selection
		for _, sel := range v.Selections {
			selections = append(selections, &pb.Listing_Item_Sku_Selection{
				Option:  sel.Option,
				Variant: sel.Variant,
			})
		}

		// Convert variant price to integer format (same as base price)
		skuPrice := v.Price
		if skuPrice != "" && skuPrice != "0" {
			priceInt, err := g.convertPriceToInt(skuPrice, 2)
			if err == nil {
				skuPrice = priceInt
			}
			// If conversion fails, use original value (may be already in integer format)
		}

		sku := &pb.Listing_Item_Sku{
			ProductID:  v.ProductID,
			Price:      skuPrice,
			Quantity:   v.Quantity,
			Barcode:    v.Barcode,
			Selections: selections,
		}

		listing.Item.Skus = append(listing.Item.Skus, sku)
	}
}

// readZipFile reads content from a zip file entry
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// convertPriceToInt converts a decimal price string to integer format
// e.g., "99.99" with divisibility=2 -> "9999"
// e.g., "100" with divisibility=2 -> "10000"
func (g *Gateway) convertPriceToInt(priceStr string, divisibility int) (string, error) {
	priceStr = strings.TrimSpace(priceStr)
	if priceStr == "" {
		return "", errors.New("price cannot be empty")
	}

	// Handle decimal point
	parts := strings.Split(priceStr, ".")
	if len(parts) > 2 {
		return "", errors.New("invalid price format: multiple decimal points")
	}

	intPart := parts[0]
	var decPart string
	if len(parts) == 2 {
		decPart = parts[1]
	}

	// Pad or truncate decimal part to match divisibility
	if len(decPart) < divisibility {
		decPart = decPart + strings.Repeat("0", divisibility-len(decPart))
	} else if len(decPart) > divisibility {
		decPart = decPart[:divisibility]
	}

	// Combine and remove leading zeros (except for "0")
	result := intPart + decPart
	result = strings.TrimLeft(result, "0")
	if result == "" {
		result = "0"
	}

	// Validate it's a valid integer
	if _, ok := new(big.Int).SetString(result, 10); !ok {
		return "", errors.New("invalid price: not a valid number")
	}

	return result, nil
}

// ============================================================================
// JSON Import Handler
// ============================================================================

// JSONListingInput represents a single listing in JSON import format
type JSONListingInput struct {
	Slug               string             `json:"slug"`
	ShippingProfileID  string             `json:"shippingProfileId"`
	Title              string             `json:"title"`
	ContractType       string             `json:"contractType"`
	Price              string             `json:"price"`
	PricingCurrency    string             `json:"pricingCurrency"`
	Description        string             `json:"description"`
	ShortDescription   string             `json:"shortDescription"`
	ProductType        string             `json:"productType"`
	Tags               []string           `json:"tags"`
	Condition          string             `json:"condition"`
	NSFW               bool               `json:"nsfw"`
	Images             []string           `json:"images"`
	IntroVideo         string             `json:"introVideo"`
	ProcessingTime     string             `json:"processingTime"`
	Grams              uint32             `json:"grams"`
	Variants []JSONVariantInput `json:"variants"`
	Quantity           string             `json:"quantity"`

	// RWA Token fields
	RwaTokenAddress         string   `json:"rwaTokenAddress"`
	RwaTokenStandard        string   `json:"rwaTokenStandard"`
	RwaTokenId              string   `json:"rwaTokenId"`
	RwaSlotId               string   `json:"rwaSlotId"` // ERC3525 专用
	RwaBlockchain           string   `json:"rwaBlockchain"`
	RwaTradeMode            int      `json:"rwaTradeMode"`
	RwaEscrowTimeoutSeconds uint64   `json:"rwaEscrowTimeoutSeconds"`
	RwaAcceptedCurrencies   []string `json:"rwaAcceptedCurrencies"`
}

// JSONVariantInput represents a variant/SKU in JSON import format
type JSONVariantInput struct {
	Selections map[string]string `json:"selections"` // e.g., {"Color": "Red", "Size": "S"}
	Price      string            `json:"price"`      // 变体绝对价格（Shopify 风格）
	Quantity   string            `json:"quantity"`
	ProductID  string            `json:"productID"`
}

// JSONImportShippingProfile is a shipping profile to create during import.
// The "key" field is a local reference that listings use in shippingProfileId
// to link to this profile (replaced with the real ID after creation).
type JSONImportShippingProfile struct {
	Key             string              `json:"key"`
	Name            string              `json:"name"`
	IsDefault       bool                `json:"isDefault"`
	LocationGroups  json.RawMessage     `json:"locationGroups"`
}

// JSONImportCollection is a collection to create during import.
// Products are referenced by listing slug.
type JSONImportCollection struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Image       string   `json:"image,omitempty"`
	SortOrder   string   `json:"sortOrder,omitempty"`
	Published   *bool    `json:"published,omitempty"`
	Products    []string `json:"products"` // listing slugs
}

// JSONImportProfile contains only the writable fields of a store profile.
// Read-only fields (peerID, publicKey, stats, avatarHashes, headerHashes)
// are intentionally excluded.
type JSONImportProfile struct {
	Name             string                     `json:"name"`
	Location         string                     `json:"location,omitempty"`
	About            string                     `json:"about,omitempty"`
	ShortDescription string                     `json:"shortDescription,omitempty"`
	Nsfw             bool                       `json:"nsfw,omitempty"`
	Vendor           *bool                      `json:"vendor,omitempty"`
	Moderator        bool                       `json:"moderator,omitempty"`
	Visibility       models.StoreVisibility     `json:"visibility,omitempty"`
	ModeratorInfo    *models.ModeratorInfo      `json:"moderatorInfo,omitempty"`
	Colors           *models.ProfileColors      `json:"colors,omitempty"`
	ContactInfo      *models.ProfileContactInfo `json:"contactInfo,omitempty"`
}

// JSONImportPayload represents the root JSON structure
type JSONImportPayload struct {
	Profile          *JSONImportProfile          `json:"profile,omitempty"`
	Listings         []JSONListingInput          `json:"listings"`
	ShippingProfiles []JSONImportShippingProfile `json:"shippingProfiles,omitempty"`
	Collections      []JSONImportCollection      `json:"collections,omitempty"`
}

// handlePOSTListingsImportJSON handles the batch import of listings from a ZIP file with JSON data.
// This is a dedicated JSON endpoint kept for backward compatibility (import.sh uses it).
func (g *Gateway) handlePOSTListingsImportJSON(w http.ResponseWriter, r *http.Request) {
	maxZipSize := g.nodeManager.GetMaxImportZipSize()
	maxVideoSize := g.nodeManager.GetMaxImportVideoSize()

	r.Body = http.MaxBytesReader(w, r.Body, maxZipSize)

	if err := r.ParseMultipartForm(maxZipSize); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("file too large or invalid: %s", err.Error()))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading file: %s", err.Error()))
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		ErrorResponse(w, http.StatusBadRequest, "file must be a ZIP archive")
		return
	}

	zipData, err := io.ReadAll(file)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading file: %s", err.Error()))
		return
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("invalid ZIP file: %s", err.Error()))
		return
	}

	var jsonFile *zip.File
	var avatarData, headerData []byte
	images := make(map[string][]byte)
	videos := make(map[string][]byte)

	for _, f := range zipReader.File {
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX") {
			continue
		}

		normalizedName := strings.TrimPrefix(strings.ToLower(f.Name), "./")
		filename := filepath.Base(f.Name)

		if slot, ok := isProfileImage(normalizedName, filename); ok {
			if data, err := readZipFile(f); err == nil {
				if slot == "avatar" {
					avatarData = data
				} else {
					headerData = data
				}
			}
			continue
		}

		switch {
		case strings.HasSuffix(normalizedName, "listings.json") || strings.HasSuffix(normalizedName, ".json"):
			if jsonFile == nil {
				jsonFile = f
			}
		case strings.HasPrefix(normalizedName, "images/") || strings.Contains(normalizedName, "/images/"):
			if data, err := readZipFile(f); err == nil {
				images[filename] = data
			}
		case strings.HasPrefix(normalizedName, "videos/") || strings.Contains(normalizedName, "/videos/"):
			if data, err := readZipFile(f); err == nil && int64(len(data)) <= maxVideoSize {
				videos[filename] = data
			}
		}
	}

	if jsonFile == nil {
		ErrorResponse(w, http.StatusBadRequest, "no JSON file found in ZIP archive")
		return
	}

	g.importFromJSON(w, r, jsonFile, images, videos, avatarData, headerData)
}

// importFromJSON is the shared JSON import logic used by both the auto-detect
// endpoint (/v1/listings/import) and the dedicated JSON endpoint (/v1/listings/import/json).
func (g *Gateway) importFromJSON(w http.ResponseWriter, r *http.Request, jsonZipEntry *zip.File, images, videos map[string][]byte, avatarData, headerData []byte) {
	jsonData, err := readZipFile(jsonZipEntry)
	if err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error reading JSON file: %s", err.Error()))
		return
	}

	var payload JSONImportPayload
	if err := json.Unmarshal(jsonData, &payload); err != nil {
		ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("error parsing JSON file: %s", err.Error()))
		return
	}

	listingSvc := getListingService(r)
	mediaSvc := getMediaService(r)

	// Phase 0: Import profile (avatar, header, profile data).
	// Failures are non-fatal — they get recorded in the result but do not
	// block the subsequent listings import.
	var profileImported bool
	var profileError string

	// Step 1: Ensure profile exists before uploading media, because
	// SetProfileMedia associates hashes with an existing profile record.
	if payload.Profile != nil && payload.Profile.Name != "" {
		prof := getProfileService(r)
		profile := models.Profile{
			Name:             payload.Profile.Name,
			Location:         payload.Profile.Location,
			About:            payload.Profile.About,
			ShortDescription: payload.Profile.ShortDescription,
			Nsfw:             payload.Profile.Nsfw,
			Moderator:        payload.Profile.Moderator,
			Visibility:       payload.Profile.Visibility,
			ModeratorInfo:    payload.Profile.ModeratorInfo,
		}
		if profile.Moderator && profile.ModeratorInfo == nil {
			profile.Moderator = false
		}
		if payload.Profile.Vendor != nil {
			profile.Vendor = *payload.Profile.Vendor
		} else {
			profile.Vendor = true
		}
		if payload.Profile.Colors != nil {
			profile.Colors = *payload.Profile.Colors
		}
		if payload.Profile.ContactInfo != nil {
			profile.ContactInfo = payload.Profile.ContactInfo
		}

		if err := prof.SetProfile(&profile, nil); err != nil {
			profileError = fmt.Sprintf("profile set failed: %s", err.Error())
		} else {
			profileImported = true
		}
	}

	// Step 2: Upload avatar/header after profile exists so hashes get associated.
	if avatarData != nil {
		if _, err := mediaSvc.SetProfileMedia(r.Context(), contracts.SlotAvatar, avatarData); err != nil {
			if profileError != "" {
				profileError += "; "
			}
			profileError += fmt.Sprintf("avatar upload failed: %s", err.Error())
		} else {
			profileImported = true
		}
	}
	if headerData != nil {
		if _, err := mediaSvc.SetProfileMedia(r.Context(), contracts.SlotHeader, headerData); err != nil {
			if profileError != "" {
				profileError += "; "
			}
			profileError += fmt.Sprintf("header upload failed: %s", err.Error())
		} else {
			profileImported = true
		}
	}

	// Phase 1: Create shipping profiles and build key→ID mapping
	profileKeyToID := make(map[string]string)
	if shippingSvc, ok := getShippingService(r); ok {
		for _, sp := range payload.ShippingProfiles {
			profile := models.ShippingProfileEntity{
				Name:      sp.Name,
				IsDefault: sp.IsDefault,
			}
			if len(sp.LocationGroups) > 0 {
				profile.LocationGroupsJSON = string(sp.LocationGroups)
			}
			if err := shippingSvc.CreateProfile(r.Context(), &profile); err != nil {
				ErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("failed to create shipping profile %q: %s", sp.Name, err.Error()))
				return
			}
			if sp.Key != "" {
				profileKeyToID[sp.Key] = profile.ID
			}
		}

		if existingProfiles, err := shippingSvc.ListProfiles(r.Context()); err == nil {
			for _, p := range existingProfiles {
				if _, exists := profileKeyToID[p.Name]; !exists {
					profileKeyToID[p.Name] = p.ID
				}
			}
		}
	}

	for i := range payload.Listings {
		ref := payload.Listings[i].ShippingProfileID
		if ref == "" {
			continue
		}
		if realID, ok := profileKeyToID[ref]; ok {
			payload.Listings[i].ShippingProfileID = realID
		}
	}

	var defaultShippingPB *pb.ShippingProfile
	if shippingSvc, ok := getShippingService(r); ok {
		if profiles, err := shippingSvc.ListProfiles(r.Context()); err == nil {
			for _, p := range profiles {
				if p.IsDefault {
					defaultShippingPB = models.ConvertShippingEntityToProto(p)
					break
				}
			}
		}
	}

	// Phase 2: Import listings
	result, err := g.processListingsImportJSON(listingSvc, mediaSvc, payload.Listings, images, videos, defaultShippingPB)
	if err != nil {
		ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	result.ProfileImported = profileImported
	result.ProfileError = profileError

	// Phase 3: Create collections
	if collSvc, ok := getCollectionService(r); ok && len(payload.Collections) > 0 {
		for _, ic := range payload.Collections {
			coll := &models.Collection{
				Title:       ic.Title,
				Description: ic.Description,
				Image:       ic.Image,
				Type:        models.CollectionTypeManual,
				SortOrder:   models.CollectionSortManual,
				Published:   true,
			}
			if ic.SortOrder != "" {
				coll.SortOrder = models.CollectionSortOrder(ic.SortOrder)
			}
			if ic.Published != nil {
				coll.Published = *ic.Published
			}
			if err := collSvc.CreateCollection(r.Context(), coll); err != nil {
				result.Errors = append(result.Errors, ImportError{
					Title: ic.Title,
					Error: fmt.Sprintf("collection create failed: %s", err.Error()),
				})
				continue
			}
			if len(ic.Products) > 0 {
				if err := collSvc.AddProducts(r.Context(), coll.ID, ic.Products); err != nil {
					result.Errors = append(result.Errors, ImportError{
						Title: ic.Title,
						Error: fmt.Sprintf("collection add products failed: %s", err.Error()),
					})
				}
			}
		}
	}

	sanitizedJSONResponse(w, result)
}

// processListingsImportJSON processes the JSON data and creates/updates listings.
// defaultShippingPB is auto-injected into PHYSICAL_GOOD listings that have no ShippingProfile.
func (g *Gateway) processListingsImportJSON(listingSvc contracts.ListingService, mediaSvc contracts.MediaService, listings []JSONListingInput, images, videos map[string][]byte, defaultShippingPB *pb.ShippingProfile) (*ImportResult, error) {
	result := &ImportResult{
		CreatedItems: []ImportedItem{},
		UpdatedItems: []ImportedItem{},
		Errors:       []ImportError{},
	}

	// Get existing listings for duplicate detection
	existingListings, _ := listingSvc.GetMyListings()
	existingByTitle := make(map[string]string) // title -> slug
	existingSlugs := make(map[string]bool)
	for _, listing := range existingListings {
		existingByTitle[listing.Title] = listing.Slug
		existingSlugs[listing.Slug] = true
	}

	// Process each listing
	for i, input := range listings {
		rowNum := i + 1

		listing, err := g.parseJSONListing(input)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: input.Title,
				Error: err.Error(),
			})
			result.Failed++
			continue
		}

		if input.Slug != "" {
			listing.Slug = input.Slug
		}

		// Process images
		if imgErr := g.processJSONListingImages(mediaSvc, listing, input.Images, images); imgErr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: imgErr.Error(),
			})
			result.Failed++
			continue
		}

		// Process intro video
		if input.IntroVideo != "" {
			if vidErr := g.processJSONListingVideo(mediaSvc, listing, input.IntroVideo, videos); vidErr != nil {
				result.Errors = append(result.Errors, ImportError{
					Row:   rowNum,
					Title: listing.Item.Title,
					Error: vidErr.Error(),
				})
				result.Failed++
				continue
			}
		}

		// Process variants
		g.processJSONListingVariants(listing, input.Variants)

		// Validate: at least one image is required
		if len(listing.Item.Images) == 0 {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: "at least one image is required",
			})
			result.Failed++
			continue
		}

		// Auto-inject default shipping profile for physical goods that have no explicit profile
		if listing.Metadata.ContractType == pb.Listing_Metadata_PHYSICAL_GOOD &&
			listing.ShippingProfile == nil && listing.ShippingProfileId == "" {
			if defaultShippingPB != nil {
				listing.ShippingProfile = defaultShippingPB
			} else {
				result.Errors = append(result.Errors, ImportError{
					Row:   rowNum,
					Title: listing.Item.Title,
					Error: "physical good requires a shipping profile; create a default shipping profile first or specify shippingProfileId",
				})
				result.Failed++
				continue
			}
		}

		// Check if listing exists (by explicit slug first, then by title)
		isUpdate := false
		if listing.Slug != "" && existingSlugs[listing.Slug] {
			isUpdate = true
		} else if listing.Slug == "" {
			if existingSlug, exists := existingByTitle[listing.Item.Title]; exists {
				listing.Slug = existingSlug
				isUpdate = true
			}
		}

		// Save listing
		saveErr := listingSvc.SaveListing(listing, nil)

		if saveErr != nil {
			result.Errors = append(result.Errors, ImportError{
				Row:   rowNum,
				Title: listing.Item.Title,
				Error: saveErr.Error(),
			})
			result.Failed++
			continue
		}

		if isUpdate {
			result.Updated++
			result.UpdatedItems = append(result.UpdatedItems, ImportedItem{
				Slug:  listing.Slug,
				Title: listing.Item.Title,
			})
		} else {
			result.Created++
			result.CreatedItems = append(result.CreatedItems, ImportedItem{
				Slug:  listing.Slug,
				Title: listing.Item.Title,
			})
			existingByTitle[listing.Item.Title] = listing.Slug
			existingSlugs[listing.Slug] = true
		}

		result.Total++
	}

	return result, nil
}

// parseJSONListing converts a JSONListingInput to a pb.Listing
func (g *Gateway) parseJSONListing(input JSONListingInput) (*pb.Listing, error) {
	if input.Title == "" {
		return nil, errors.New("title is required")
	}

	if input.Price == "" {
		return nil, errors.New("price is required")
	}

	currency := input.PricingCurrency
	if currency == "" {
		currency = "USD"
	}

	// Convert price to integer format
	priceInt, err := g.convertPriceToInt(input.Price, 2)
	if err != nil {
		return nil, fmt.Errorf("invalid price format: %w", err)
	}

	contractType := g.parseContractType(input.ContractType)

	listing := &pb.Listing{
		ShippingProfileId: input.ShippingProfileID,
		Metadata: &pb.Listing_Metadata{
			ContractType: contractType,
			Format:       pb.Listing_Metadata_FIXED_PRICE,
			PricingCurrency: &pb.Currency{
				Code:         strings.ToUpper(currency),
				Divisibility: 2,
			},
			Expiry: timestamppb.New(time.Date(2037, 12, 31, 0, 0, 0, 0, time.UTC)),
		},
		Item: &pb.Listing_Item{
			Title:            input.Title,
			Description:      input.Description,
			ShortDescription: input.ShortDescription,
			Price:            priceInt,
			Condition:        input.Condition,
			ProcessingTime:   input.ProcessingTime,
			Nsfw:             input.NSFW,
			ProductType:      input.ProductType,
			Tags:             input.Tags,
			Grams:            input.Grams,
		},
	}

	// Handle RWA Token fields
	if contractType == pb.Listing_Metadata_RWA_TOKEN {
		// Set RWA metadata
		listing.Metadata.AcceptedCurrencies = input.RwaAcceptedCurrencies
		listing.Metadata.RwaTradeMode = pb.Listing_Metadata_RwaTradeMode(input.RwaTradeMode)
		if input.RwaEscrowTimeoutSeconds > 0 {
			listing.Metadata.RwaEscrowTimeoutSeconds = input.RwaEscrowTimeoutSeconds
		} else {
			listing.Metadata.RwaEscrowTimeoutSeconds = 86400 // Default 24 hours
		}

		// Set RWA item fields
		listing.Item.Blockchain = input.RwaBlockchain
		listing.Item.TokenAddress = input.RwaTokenAddress
		listing.Item.TokenStandard = input.RwaTokenStandard
		listing.Item.TokenId = input.RwaTokenId
		listing.Item.SlotId = input.RwaSlotId

		// Generate cryptoListingCurrencyCode
		// Format: CHAIN_ADDRESS_STANDARD_ID
		// - ERC721/ERC1155: use tokenId
		// - ERC3525: use slotId
		chain := strings.ToUpper(input.RwaBlockchain)
		addr := strings.ToLower(input.RwaTokenAddress)
		standard := strings.ToUpper(input.RwaTokenStandard)

		var id string
		if standard == "ERC3525" {
			id = input.RwaSlotId
		} else {
			id = input.RwaTokenId
		}

		if chain != "" && addr != "" && standard != "" && id != "" {
			listing.Item.CryptoListingCurrencyCode = fmt.Sprintf("%s_%s_%s_%s", chain, addr, standard, id)
		}
	}

	return listing, nil
}

// processJSONListingImages processes and uploads images for a listing from JSON input
func (g *Gateway) processJSONListingImages(mediaSvc contracts.MediaService, listing *pb.Listing, imageNames []string, images map[string][]byte) error {
	for _, imgName := range imageNames {
		imgName = strings.TrimSpace(imgName)
		if imgName == "" {
			continue
		}

		imgData, ok := images[imgName]
		if !ok {
			// Try to find with case-insensitive match
			for name, data := range images {
				if strings.EqualFold(name, imgName) {
					imgData = data
					ok = true
					break
				}
			}
		}

		if !ok {
			continue // Skip missing images
		}

		// Check if it's a valid image
		if !filetype.IsImage(imgData) {
			continue
		}

		// Upload image with variants
		result, err := mediaSvc.UploadMedia(context.Background(), imgData, imgName, contracts.UploadOpts{Variants: true})
		if err != nil {
			return fmt.Errorf("failed to upload image %s: %w", imgName, err)
		}

		if result.Hashes != nil {
			listing.Item.Images = append(listing.Item.Images, &pb.Image{
				Filename: imgName,
				Original: result.Hashes.Original,
				Large:    result.Hashes.Large,
				Medium:   result.Hashes.Medium,
				Small:    result.Hashes.Small,
				Tiny:     result.Hashes.Tiny,
			})
		}
	}

	return nil
}

// processJSONListingVideo processes and uploads intro video for a listing from JSON input
func (g *Gateway) processJSONListingVideo(mediaSvc contracts.MediaService, listing *pb.Listing, videoName string, videos map[string][]byte) error {
	videoName = strings.TrimSpace(videoName)
	if videoName == "" {
		return nil
	}

	videoData, ok := videos[videoName]
	if !ok {
		// Try case-insensitive match
		for name, data := range videos {
			if strings.EqualFold(name, videoName) {
				videoData = data
				ok = true
				break
			}
		}
	}

	if !ok {
		return nil // Skip missing video
	}

	// Check video size
	maxVideoSize := g.nodeManager.GetMaxImportVideoSize()
	if int64(len(videoData)) > maxVideoSize {
		return fmt.Errorf("video %s exceeds maximum size of %dMB", videoName, maxVideoSize/(1<<20))
	}

	// Check if it's a valid video
	if !filetype.IsVideo(videoData) {
		return fmt.Errorf("file %s is not a valid video", videoName)
	}

	// Upload video
	result, err := mediaSvc.UploadMedia(context.Background(), videoData, videoName, contracts.UploadOpts{MaxBytes: maxVideoSize})
	if err != nil {
		return fmt.Errorf("failed to upload video %s: %w", videoName, err)
	}

	listing.Item.IntroVideo = &pb.File{
		Filename: videoName,
		Hash:     result.Hash,
	}

	return nil
}

// processJSONListingVariants adds variants to a listing from JSON input
func (g *Gateway) processJSONListingVariants(listing *pb.Listing, variants []JSONVariantInput) {
	if len(variants) == 0 {
		return
	}

	// Collect all options and their variants
	optionVariants := make(map[string]map[string]bool)

	for _, v := range variants {
		for optName, variantName := range v.Selections {
			if optionVariants[optName] == nil {
				optionVariants[optName] = make(map[string]bool)
			}
			optionVariants[optName][variantName] = true
		}
	}

	// Create options with their variants
	for optName, variantSet := range optionVariants {
		option := &pb.Listing_Item_Option{
			Name: optName,
		}

		for variantName := range variantSet {
			option.Variants = append(option.Variants, &pb.Listing_Item_Option_Variant{
				Name: variantName,
			})
		}

		listing.Item.Options = append(listing.Item.Options, option)
	}

	// Create SKUs from variants
	for _, v := range variants {
		var selections []*pb.Listing_Item_Sku_Selection
		for optName, variantName := range v.Selections {
			selections = append(selections, &pb.Listing_Item_Sku_Selection{
				Option:  optName,
				Variant: variantName,
			})
		}

		sku := &pb.Listing_Item_Sku{
			ProductID:  v.ProductID,
			Price:      v.Price,
			Quantity:   v.Quantity,
			Selections: selections,
		}

		listing.Item.Skus = append(listing.Item.Skus, sku)
	}
}
