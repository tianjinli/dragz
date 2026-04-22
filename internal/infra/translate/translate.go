package translate

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/locales"
	"github.com/go-playground/locales/en_US"
	"github.com/go-playground/locales/zh_Hans"
	"github.com/go-playground/locales/zh_Hant"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/go-playground/validator/v10/translations/en"
	"github.com/go-playground/validator/v10/translations/zh"
	"github.com/go-playground/validator/v10/translations/zh_tw"
	"github.com/tianjinli/dragz/internal/i18n"
	"github.com/tianjinli/dragz/pkg/appkit"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

const AcceptLanguage = "Accept-Language"

type TranslationMap map[string]map[string]string

var supportLocales = []string{"en_US", "zh_Hans", "zh_Hant"}

type translateService struct {
	matcher    language.Matcher
	translator *ut.UniversalTranslator
}

func tagToLocale(tag language.Tag) string {
	return strings.Replace(tag.String(), "-", "_", -1)
}

func (s *translateService) GetTranslator(lang string) (ut.Translator, bool) {
	// zh-CN;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3
	if lang == "" {
		return s.translator.GetFallback(), false
	}
	acceptTags, _, _ := language.ParseAcceptLanguage(lang)   // [zh-CN zh-TW zh-HK en-US]
	_, index, _ := s.matcher.Match(acceptTags...)            // zh-Hans-u-rg-cnzzzz
	return s.translator.GetTranslator(supportLocales[index]) // zh_Hans

	//return s.translator.FindTranslator(pure.AcceptedLanguages(c.Request)...)
}

func (s *translateService) RenderError(ctx *gin.Context, err error) {
	lang := ctx.GetHeader(AcceptLanguage)
	trans, _ := s.GetTranslator(lang)
	var ie *appkit.I18nError
	if errors.As(err, &ie) {
		ctx.JSON(ie.StatusCode, ie.Translate(trans))
		return
	}

	var errs validator.ValidationErrors
	if errors.As(err, &errs) {
		translated := errs.Translate(trans)
		for _, val := range translated {
			ctx.JSON(http.StatusBadRequest, appkit.ErrorResponse{Code: i18n.ErrBaseInvalidParams, Message: val})
			return
		}
	}

	ie = appkit.NewBadRequest(i18n.ErrBaseBadRequest).WithError(err)
	ctx.JSON(ie.StatusCode, ie.Translate(trans))
}

func (s *translateService) RegisterTranslations(validate *validator.Validate) (err error) {
	if trans, found := s.translator.GetTranslator(tagToLocale(language.AmericanEnglish)); found {
		if err = en.RegisterDefaultTranslations(validate, trans); err != nil {
			return err
		}
	}
	if trans, found := s.translator.GetTranslator(tagToLocale(language.SimplifiedChinese)); found {
		if err = zh.RegisterDefaultTranslations(validate, trans); err != nil {
			return err
		}
	}
	if trans, found := s.translator.GetTranslator(tagToLocale(language.TraditionalChinese)); found {
		if err = zh_tw.RegisterDefaultTranslations(validate, trans); err != nil {
			return err
		}
	}
	return nil
}

func (s *translateService) ResolveTranslations(localesDir string, rootNode string) (err error) {
	if trans, found := s.translator.GetTranslator(tagToLocale(language.AmericanEnglish)); found {
		if err = s.ResolveLangFile(localesDir, rootNode, language.AmericanEnglish, trans); err != nil {
			return err
		}
	}
	if trans, found := s.translator.GetTranslator(tagToLocale(language.SimplifiedChinese)); found {
		if err = s.ResolveLangFile(localesDir, rootNode, language.SimplifiedChinese, trans); err != nil {
			return err
		}
	}
	if trans, found := s.translator.GetTranslator(tagToLocale(language.TraditionalChinese)); found {
		if err = s.ResolveLangFile(localesDir, rootNode, language.TraditionalChinese, trans); err != nil {
			return err
		}
	}
	return nil
}

func (s *translateService) resolveLangData(rootNode string, contents []byte, trans ut.Translator) (err error) {
	var translations TranslationMap
	if rootNode != "" {
		var out map[string]TranslationMap
		if err = yaml.Unmarshal(contents, &out); err != nil {
			return err
		}
		translations = out[rootNode]
	} else {
		if err = yaml.Unmarshal(contents, &translations); err != nil {
			return err
		}
	}
	if rootNode != "" {
		for key, value := range translations {
			for subKey, subValue := range value {
				if err = trans.Add(fmt.Sprintf("%s.%s.%s", rootNode, key, subKey), subValue, true); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for key, value := range translations {
		for subKey, subValue := range value {
			if err = trans.Add(fmt.Sprintf("%s.%s", key, subKey), subValue, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *translateService) ResolveLangFile(localesDir string, rootNode string, tag language.Tag, trans ut.Translator) (err error) {
	base, _ := tag.Base()
	region, _ := tag.Region()
	var contents []byte
	filename := fmt.Sprintf("%s_%s.yaml", base.String(), region.String())
	if localesDir == "" {
		contents, err = i18n.LocalesFS.ReadFile(fmt.Sprintf("locales/%s", filename))
		if err != nil {
			return err
		}
	} else {
		contents, err = os.ReadFile(filepath.Join(localesDir, filename))
		if err != nil {
			return err
		}
	}
	return s.resolveLangData(rootNode, contents, trans)
}

// NewI18nAdapter creates a new infra.I18nAdapter.
func NewI18nAdapter(conf *appkit.ServerConfig) (appkit.I18nAdapter, error) {
	acceptTags, _, _ := language.ParseAcceptLanguage(conf.Locale) // [zh-CN zh-TW zh-HK en-US]
	matcher := language.NewMatcher([]language.Tag{language.AmericanEnglish, language.SimplifiedChinese, language.TraditionalChinese})
	_, index, _ := matcher.Match(acceptTags...) // zh-Hans-u-rg-cnzzzz
	var trans locales.Translator
	switch index {
	case 1:
		trans = zh_Hans.New()
	case 2:
		trans = zh_Hant.New()
	default:
		trans = en_US.New()
	}
	translator := ut.New(trans, en_US.New(), zh_Hans.New(), zh_Hant.New())
	service := &translateService{matcher: matcher, translator: translator}
	return service, nil
}
