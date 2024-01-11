package clean

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/rs/zerolog"
)

var log zerolog.Logger

func init() {
	var output io.Writer = zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
		FormatLevel: func(i interface{}) string {
			return strings.ToUpper(fmt.Sprintf("[%s]", i))
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("| %s |", i)
		},
		// FormatCaller: func(i interface{}) string {
		// 	return filepath.Base(fmt.Sprintf("%s", i))
		// },
		PartsExclude: []string{
			zerolog.TimestampFieldName,
		},
	}
	if os.Getenv("GO_ENV") != "development" {
		output = os.Stderr
	}
	log = zerolog.New(output)
}

func CheckImageNotInUse(all []*ImageInfo, detail types.ImageDetail) bool {
	for _, image := range all {
		_, deetsDigest, _ := strings.Cut(*detail.ImageDigest, ":")
		if deetsDigest == image.Digest {
			return false
		}
	}
	return true
}

func GetPrometheusImagesFromProfile() ([]*ImageInfo, error) {
	allImages := make([]*ImageInfo, 0)
	datasource := Datasource{
		UID: "W7Xb02Snk",
	}
	query := Queries{
		RefID: "A",
		Expr:  "kube_pod_container_info",

		Datasource: datasource,
	}
	postBody := PostBody{
		From:    "now-1h",
		Queries: []Queries{query},
		To:      "now",
	}
	buf, err := json.Marshal(postBody)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshall json")
	}
	grafanaApiKey := os.Getenv("GRAFANA_API_KEY")
	if grafanaApiKey == "" {
		err = errors.New("env var GRAFANA_API_KEY not set")
		log.Error().Err(err).Msg("failed to load environment variable")
		return nil, err
	}
	req, err := http.NewRequest("POST", "https://grafana.ltvops.com/api/ds/query", bytes.NewBuffer(buf))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", grafanaApiKey))
	req.Header.Add("Content-Type", "application/json")
	if err != nil {
		log.Error().Err(err).Msg("failed to build request")
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("failed to call grafana")
		return nil, err
	}
	if resp.StatusCode != 200 {
		err = errors.New("non-200 status code")
		log.Error().Err(err).Int("statusCode", resp.StatusCode).Str("status", resp.Status).Msg("failed to get a 200 on prometheus query")
		return nil, err
	}
	defer resp.Body.Close()

	body := &Response{}
	json.NewDecoder(resp.Body).Decode(body)
	for _, item := range body.Results.A.Frames {
		for _, field := range item.Schema.Fields {
			if field.Labels.Name == "kube_pod_container_info" {
				registryUrl := strings.Split(strings.Split(field.Labels.Image, "@")[0], ":")[0]
				if field.Labels.ImageID != "" {
					digest := strings.Split(strings.Split(field.Labels.ImageID, "@")[1], ":")[1]
					newImage := ImageInfo{
						FullImagePath:  field.Labels.Image,
						RegistryUrl:    registryUrl,
						RepositoryName: strings.Split(registryUrl, "/")[len(strings.Split(registryUrl, "/"))-1],
						DeployedTag:    strings.Split(field.Labels.Image, ":")[1],
						Digest:         digest,
						Cluster:        field.Labels.Cluster,
					}
					allImages = append(allImages, &newImage)
				}
			}
		}
	}
	return allImages, nil
}

func GetECRImages(client *ecr.Client) ([]*ecr.DescribeImagesOutput, error) {
	var allECRImages []*ecr.DescribeImagesOutput
	registry, err := client.DescribeRegistry(context.Background(), &ecr.DescribeRegistryInput{})
	if err != nil {
		log.Error().Err(err).Msg("failed to describe registry")
		return nil, err
	}
	log.Debug().Str("awsAccount", *registry.RegistryId).Msg("AWS Account number to be scanned")
	// fmt.Printf("repos.RegistryId: %v\n", *registry.RegistryId)
	repos, err := client.DescribeRepositories(context.Background(), &ecr.DescribeRepositoriesInput{})
	if err != nil {
		log.Error().Err(err).Msg("failed to describe repositories")
		return nil, err
	}
	for _, repo := range repos.Repositories {
		if repo.RepositoryName != nil {
			input := &ecr.ListImagesInput{
				RepositoryName: repo.RepositoryName,
			}
			images, err := client.ListImages(context.Background(), input)
			if err != nil {
				log.Error().Err(err).Msg("failed to list images")
				return nil, err
			}
			if len(images.ImageIds) > 0 {
				describe, err := client.DescribeImages(context.Background(), &ecr.DescribeImagesInput{
					ImageIds:       images.ImageIds,
					RepositoryName: repo.RepositoryName,
				})
				if err != nil {
					log.Error().Err(err).Msg("failed to describe images")
					return nil, err
				}
				allECRImages = append(allECRImages, describe)
			}
		}
	}
	return allECRImages, nil
}

func GetECRImage(ctx context.Context, client *ecr.Client, image types.ImageDetail) error {
	log.Debug().Str("imageRepositoryName", *image.RepositoryName).Msg("getting image from ECR")
	data, err := client.BatchGetImage(ctx, &ecr.BatchGetImageInput{
		ImageIds: []types.ImageIdentifier{{
			ImageDigest: image.ImageDigest,
		}},
		RepositoryName: image.RepositoryName,
		RegistryId:     image.RegistryId,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to get image")
		return err
	}
	for _, v := range data.Images {
		log.Info().Interface("imageDetails", v).Msg("GetECRImage details")
	}
	return nil
}

func DeleteECRImage(ctx context.Context, client *ecr.Client, image types.ImageDetail, dryRun bool) error {
	if dryRun {
		log.Info().Interface("imageDetail", image).Msg("dry run of delete image")
		return nil
	}
	delete, err := client.BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
		ImageIds: []types.ImageIdentifier{{
			ImageDigest: image.ImageDigest,
		}},
		RepositoryName: image.RepositoryName,
		RegistryId:     image.RegistryId,
	})
	if err != nil {
		log.Error().Err(err).Str("repoName", *image.RepositoryName).Str("imageDigest", *image.ImageDigest).Msg("failed to delete image")
		return err
	}
	for _, ii := range delete.ImageIds {
		log.Info().Interface("deletedImage", ii).Msg("deleted image")
		// fmt.Printf("ImageDigest and tags deleted: %v,\n", ii.ImageDigest)
	}
	return nil
}

func GetUnique(all []*ImageInfo) []*ImageInfo {
	var unique []*ImageInfo
	for _, v := range all {
		skip := false
		for _, u := range unique {
			if v.FullImagePath == u.FullImagePath {
				skip = true
				break
			}
		}
		if !skip {
			unique = append(unique, v)
		}
	}
	return unique
}

func CleanRepos(untaggedOnly bool, keepLastCount int, profile string, region string, dryRun bool, verbose bool) bool {
	arnmap := map[string]string{
		"development": "arn:aws:iam::722014088219:role/devops-read-only",
		"production":  "arn:aws:iam::667347940230:role/devops-read-only",
	}
	log.Info().Bool("untaggedOnly", untaggedOnly).Bool("dryRun", dryRun).Int("keepLastCount", keepLastCount).Str("profile", profile).Bool("verbose", verbose).Str("region", region).Msg("starting ecr-cleanup process")
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to load SDK config")
	}
	stsSvc := sts.NewFromConfig(cfg)
	stsCred := stscreds.NewAssumeRoleProvider(stsSvc, arnmap[profile])

	cfg.Credentials = aws.NewCredentialsCache(stsCred)
	client := ecr.NewFromConfig(cfg)
	allImages, err := GetPrometheusImagesFromProfile()
	if err != nil {
		log.Error().Err(err).Msg("failed to get prometheus images")
		return false
	}

	allEcrImages, err := GetECRImages(client)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get ECR images")
		return true
	}
	log.Debug().Int("prometheusImageCount", len(allImages)).Msg("number of prometheus images before deduplication")
	// fmt.Printf("before unique allImages: %v\n", len(allImages))
	allImages = GetUnique(allImages)
	log.Debug().Int("prometheusImageCount", len(allImages)).Msg("number of prometheus images after deduplication")
	// fmt.Printf("after unique allImages: %v\n", len(allImages))
	var removeUntagged []*ecr.DescribeImagesOutput
	var keepers []*ecr.DescribeImagesOutput
	var cantDelete []*ecr.DescribeImagesOutput
	for _, output := range allEcrImages {
		untagged := new(ecr.DescribeImagesOutput)
		keeper := new(ecr.DescribeImagesOutput)
		sort.Slice(output.ImageDetails, func(i, j int) bool {
			return output.ImageDetails[i].ImagePushedAt.After(*output.ImageDetails[j].ImagePushedAt)
		})
		for _, deets := range output.ImageDetails {
			if deets.ImageTags == nil {
				untagged.ImageDetails = append(untagged.ImageDetails, deets)
				continue
			}
			// fmt.Printf("deets.ImagePushedAt: %v: %v\n", *deets.RepositoryName, *deets.ImagePushedAt)
			if !untaggedOnly {
				if len(keeper.ImageDetails) <= keepLastCount {
					keeper.ImageDetails = append(keeper.ImageDetails, deets)
					continue
				}
				untagged.ImageDetails = append(untagged.ImageDetails, deets)
			}

		}
		keepers = append(keepers, keeper)
		removeUntagged = append(removeUntagged, untagged)

	}
	for _, unt := range removeUntagged {
		noDelete := new(ecr.DescribeImagesOutput)
		for _, deet := range unt.ImageDetails {
			if deet.ImageDigest != nil {
				if CheckImageNotInUse(allImages, deet) {
					DeleteECRImage(context.Background(), client, deet, dryRun)
					if verbose {
						GetECRImage(context.Background(), client, deet)
					}
				} else {
					// fmt.Printf("*****Can't delete %v: %v because it is in use!!!!\n", *deet.RepositoryName, *deet.ImageDigest)
					log.Warn().Interface("details of image", deet).Msg("can't delete because image is in use")
					noDelete.ImageDetails = append(noDelete.ImageDetails, deet)

				}
			}
		}
		if noDelete.ImageDetails != nil {
			cantDelete = append(cantDelete, noDelete)
		}
	}
	// for _, k := range keepers {
	// 	for _, v := range k.ImageDetails {
	// 		// fmt.Printf("keeper.RepositoryName: %v:%v Tags:%v Pushed At: %v \n", *v.RepositoryName, *v.ImageDigest, v.ImageTags, v.ImagePushedAt)
	// 	}
	// }

	log.Debug().Int("numberRepos", len(keepers)).Msg("number of repositories scanned")
	// fmt.Printf("number of repos: %v\n", len(keepers))
	count := 0
	count2 := 0
	for _, dio := range keepers {
		count += len(dio.ImageDetails)

	}
	log.Info().Int("keepImagesCount", count).Msg("number of images to keep")
	// fmt.Printf("keeper images count: %v\n", count)
	for _, dio := range removeUntagged {
		count2 += len(dio.ImageDetails)
	}
	log.Info().Int("removedImagesCount", count2).Msg("total number of images removed")
	// fmt.Printf("removeUntagged images count: %v\n", count2)
	if len(cantDelete) > 0 {
		log.Info().Int("cannotDelete", len(cantDelete)).Msg("number of images that cannot be deleted due to being in use")
	}
	// fmt.Printf("cantDelete: %v\n", len(cantDelete))
	for _, k := range cantDelete {
		for _, v := range k.ImageDetails {
			log.Info().Interface("cannotDeleteImage", v).Msg("image cannot be deleted")
			// fmt.Printf("Currently in use: %v:%v Tags:%v Pushed At: %v \n", *v.RepositoryName, *v.ImageDigest, v.ImageTags, v.ImagePushedAt)
		}
	}
	return false
}
