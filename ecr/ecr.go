package ecr

import (
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
)

var service *ecr.ECR

func ecrClient() *ecr.ECR {
	if service == nil {
		config := aws.NewConfig()
		timeout := 500 * time.Millisecond
		config = config.WithHTTPClient(&http.Client{Timeout: timeout})
		service = ecr.New(session.New(config))
	}

	return service
}

func cleanRepo(repoName, tagRegex string, dryRun bool) error {
	images, err := getImages(repoName)
	if err != nil {
		return err
	}

	remove := []*ecr.ImageIdentifier{}

	for _, image := range images {
		untagged, err := isUntagged(image, tagRegex)
		if err != nil {
			return err
		}

		if untagged {
			if dryRun {
				log.Printf("[DRY RUN] Removing (%s) -> %v", repoName, stringify(image.ImageTags))
			} else {
				remove = append(remove, &ecr.ImageIdentifier{ImageDigest: image.ImageDigest})
			}
		}
	}

	if dryRun {
		return nil
	}

	return deleteImages(repoName, remove)
}

func getRepos() ([]*ecr.Repository, error) {
	svc := ecrClient()

	result, err := svc.DescribeRepositories(&ecr.DescribeRepositoriesInput{})
	if err != nil {
		return nil, err
	}

	return result.Repositories, nil
}

func getImages(repo string) ([]*ecr.ImageDetail, error) {
	svc := ecrClient()

	output := &ecr.DescribeImagesOutput{}

	err := svc.DescribeImagesPages(&ecr.DescribeImagesInput{
		RepositoryName: aws.String(repo),
	},
		func(page *ecr.DescribeImagesOutput, lastPage bool) bool {
			output.ImageDetails = append(output.ImageDetails, page.ImageDetails...)
			return !lastPage
		},
	)
	if err != nil {
		return nil, err
	}

	return output.ImageDetails, nil
}

func deleteImages(repoName string, ids []*ecr.ImageIdentifier) error {
	svc := ecrClient()

	if len(ids) == 0 {
		log.Printf("No untagged images found for: %s", repoName)
		return nil
	}

	_, err := svc.BatchDeleteImage(&ecr.BatchDeleteImageInput{
		ImageIds:       ids,
		RepositoryName: aws.String(repoName),
	})
	if err != nil {
		return err
	}

	log.Printf("Deleted untagged images for: %s", repoName)

	return nil
}

func isUntagged(id *ecr.ImageDetail, regex string) (bool, error) {
	if len(id.ImageTags) == 0 {
		return true, nil
	} else {
		for _, s := range id.ImageTags {
			matched, err := regexp.MatchString(regex, *s)
			if err != nil {
				return false, err
			}
			if !matched {
				return false, nil
			}
		}
		return true, nil
	}
	return false, nil
}

type Opts struct {
	RepoName string
	TagRegex string
	RunAll   bool
	DryRun   bool
}

func CleanRepos(opts Opts) error {
	if opts.RunAll {
		repos, err := getRepos()
		if err != nil {
			return err
		}

		for _, repo := range repos {
			err := cleanRepo(*repo.RepositoryName, opts.TagRegex, opts.DryRun)
			if err != nil {
				return err
			}
		}
	} else {
		err := cleanRepo(opts.RepoName, opts.TagRegex, opts.DryRun)
		if err != nil {
			return err
		}
	}

	return nil
}

func stringify(ss []*string) []string {
	res := []string{}
	for _, s := range ss {
		res = append(res, *s)
	}
	return res
}
