#!/bin/bash
# Usage build-sdk.sh [version-tag] [pulumi-cloud-ref-name]
#
# version-tag defaults to current date and time
# ref-name defaults to master (can be a branch or tag name)
set -o nounset -o errexit -o pipefail
SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

S3_PROD_BUCKET_ROOT="s3://get.pulumi.com/releases/"
S3_ENG_BUCKET_ROOT="s3://eng.pulumi.com/releases/"
S3_PUBLISH_FOLDER_SDK="${S3_PROD_BUCKET_ROOT}sdk/"

# This function downloads a specific release and into the current working directory
# usage: download_release <repo-name> <commitish>
function download_release()
{
    local repo_name=${1}
    local repo_commit=${2}

    echo "downloading ${repo_name}@${repo_commit}"

    local file=${repo_commit}.tgz
    local s3_file=${S3_ENG_BUCKET_ROOT}${repo_name}/${OS}/amd64/${file}

    # Use AWS CLI to download the package corresponding to the component from S3 bucket
    if ! aws s3 cp --only-show-errors "${s3_file}" "./${file}" 2>/dev/null; then
        >&2 echo "failed to download ${s3_file}"
        exit 1
    fi
}

# This function downloads and extracts a specific release and into the current working directory
# usage: download_and_extract_release <repo-name> <commitish>
function download_and_extract_release()
{
    local repo_name=${1}
    local repo_commit=${2}

    local file=${repo_commit}.tgz

    download_release "${repo_name}" "${repo_commit}"

    if ! tar -xzf "${file}" 2>/dev/null; then
        >&2 echo "failed to untar ${file}"
        exit 1
    fi

    rm "./${file}"
}

# get the OS version
OS=""
case $(uname) in
    "Linux") OS=linux;;
    "Darwin") OS=darwin;;
    *) echo "error: unknown host os $(uname)" ; exit 1;;
esac

SDK_FILENAME=pulumi-${1:-$(date +"%Y%m%d_%H%M%S")}-${OS}-x64.tar.gz
PULUMI_REF=${2:-master}

# setup temporary folder to process the package
PULUMI_FOLDER=$(mktemp -d)/pulumi
mkdir -p "${PULUMI_FOLDER}"

cd "${PULUMI_FOLDER}"

cp "${SCRIPT_DIR}/../dist/install.sh" .

download_and_extract_release pulumi "${PULUMI_REF}"

# All node packages are now delivered via npm, so remove the node_modules folder.
rm -rf "${PULUMI_FOLDER}/node_modules"

SDK_PACKAGE_PATH=$(mktemp)

echo "compressing package to ${SDK_PACKAGE_PATH}"

cd ..

if ! tar -zcf ${SDK_PACKAGE_PATH} pulumi; then
    >&2 echo "failed to compress package"
    exit 1
fi

echo "uploading SDK to ${S3_PUBLISH_FOLDER_SDK}${SDK_FILENAME}"
echo "${SDK_PACKAGE_PATH}"
# rel.pulumi.com is in our production account, so assume that role first
CREDS_JSON=$(aws sts assume-role \
                 --role-arn "arn:aws:iam::058607598222:role/UploadPulumiReleases" \
                 --role-session-name "upload-sdk" \
                 --external-id "upload-pulumi-release")

# Use these new credentials to create the PPC user account.
export AWS_ACCESS_KEY_ID=$(echo ${CREDS_JSON}     | jq ".Credentials.AccessKeyId" --raw-output)
export AWS_SECRET_ACCESS_KEY=$(echo ${CREDS_JSON} | jq ".Credentials.SecretAccessKey" --raw-output)
export AWS_SECURITY_TOKEN=$(echo ${CREDS_JSON}    | jq ".Credentials.SessionToken" --raw-output)

aws s3 cp --acl public-read --only-show-errors "${SDK_PACKAGE_PATH}" "${S3_PUBLISH_FOLDER_SDK}${SDK_FILENAME}"

rm "${SDK_PACKAGE_PATH}"
rm -rf "${PULUMI_FOLDER}/"

echo "done"
