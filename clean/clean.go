package clean

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/rs/zerolog/log"
)

func CheckImageNotInUse(all []*ImageInfo, detail types.ImageDetail) bool {
	for _, image := range all {
		// _, err := json.MarshalIndent(image, "", "\t")
		// if err != nil {
		// 	log.Error().Err(err).Msg("failed to marshalIndent json")
		// }
		// deetsDigest := strings.Split(*detail.ImageDigest, ":")[1]
		_, deetsDigest, _ := strings.Cut(*detail.ImageDigest, ":")
		// fmt.Printf("deetsDigest: %v\n", deetsDigest)
		// fmt.Printf("image.Digest: %v\n", image.Digest)
		if deetsDigest == image.Digest {

			// fmt.Printf("running image %v:%v pushed at %v\n", *deets.RepositoryName, deets.ImageTags, deets.ImagePushedAt)
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
	fmt.Printf("repos.RegistryId: %v\n", *registry.RegistryId)
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
	// delete, err := client.BatchDeleteImage(ctx, &ecr.BatchDeleteImageInput{
	// 	ImageIds: []types.ImageIdentifier{{
	// 		ImageDigest: image.ImageDigest,
	// 	}},
	// 	RepositoryName: image.RepositoryName,
	// 	RegistryId:     image.RegistryId,
	// })
	// if err != nil {
	// 	log.Error().Err(err).Str("repoName", *image.RepositoryName).Str("imageDigest", *image.ImageDigest).Msg("failed to delete image")
	// }
	// for _, ii := range delete.ImageIds {
	// 	fmt.Printf("ii.ImageDigest: %v\n", ii.ImageDigest)
	// }
	for _, i2 := range data.Images {

		if i2.ImageId.ImageTag != nil {
			// fmt.Printf("RepoName:ImageDigest and tags: %v:%v %v\n", *i2.RepositoryName, *i2.ImageId.ImageDigest, *i2.ImageId.ImageTag)
			continue
		}
		// fmt.Printf("RepoName:Digest: %v:%v\n", *i2.RepositoryName, *i2.ImageId.ImageDigest)
	}
	return nil
}

func DeleteECRImage(ctx context.Context, client *ecr.Client, image types.ImageDetail) error {
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
		fmt.Printf("ImageDigest and tags deleted: %v,\n", ii.ImageDigest)
	}
	// for _, i2 := range data.Images {
	// 	fmt.Printf("i2.RepositoryName: %v\n", *i2.RepositoryName)
	// 	if i2.ImageId.ImageTag != nil {
	// 		fmt.Printf("i2.ImageId.ImageTag: %v\n", *i2.ImageId.ImageTag)
	// 	}
	// }
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

func CleanRepos(untaggedOnly bool, keepLastCount int, profile string, region string, dryRun bool) bool {
	// fmt.Printf("Cleaning %v profile of following images: \n\tUntagged Only: %v\n\tKeeping Last: %v\n\tDry Run: %v\n", profile, untaggedOnly, keepLastCount, dryRun)
	log.Info().Bool("untaggedOnly", untaggedOnly).Bool("dryRun", dryRun).Int("keepLastCount", keepLastCount).Str("profile", profile).Msg("starting ecr-cleanup process")
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region), config.WithSharedConfigProfile(profile))
	if err != nil {
		log.Fatal().Err(err).Msg("unable to load SDK config")
	}
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
					if dryRun {
						GetECRImage(context.Background(), client, deet)
						continue
					}
					DeleteECRImage(context.Background(), client, deet)
				} else {
					fmt.Printf("*****Can't delete %v: %v because it is in use!!!!\n", *deet.RepositoryName, *deet.ImageDigest)
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

	fmt.Printf("number of repos: %v\n", len(keepers))
	count := 0
	count2 := 0
	for _, dio := range keepers {
		count += len(dio.ImageDetails)

	}
	fmt.Printf("keeper images count: %v\n", count)
	for _, dio := range removeUntagged {
		count2 += len(dio.ImageDetails)
	}
	fmt.Printf("removeUntagged images count: %v\n", count2)
	fmt.Printf("cantDelete: %v\n", len(cantDelete))
	for _, k := range cantDelete {
		for _, v := range k.ImageDetails {
			fmt.Printf("Currently in use: %v:%v Tags:%v Pushed At: %v \n", *v.RepositoryName, *v.ImageDigest, v.ImageTags, v.ImagePushedAt)
		}
	}
	return false
}
