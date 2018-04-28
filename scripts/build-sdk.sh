#!/bin/bash
# Usage CreatePkg.sh [version-tag] [ref-name]
# CreatePkg.sh will package the latest packages from S3 release bucket based on the git commit associated with ref-name
# for main components  namely pulumi, pulumi-aws, pulumi-azure, pulumi-kubernetes, pulumi-cloud and then package them together along with install.sh to
# create a distributable SDK package.
#
# version-tag defaults to current date and time
# ref-name defaults to master (can be a branch or tag name)
set -o nounset -o errexit -o pipefail
SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"

# This function does npm install of the extracted packages
# usage npm_install path_to_module
function npm_install()
{
    echo Installing Node modules from ${1}
    ( cd ${1} ; npm install --only=production )
}

# This function does the following
# Usage process_package <component_name> <s3_bucket_package_folder>
# 1. Gets the last commit for the component identified by the repo
# 2. Get the tgz package corresponding to the commit from the S3 bucket
# 3. Extracts the package locally 
# 4. Deletes the package after successful extraction

function process_package()
{
    echo Getting Packages for repo: ${1}
    commit=$(git ls-remote -h -t git@github.com:pulumi/$1.git ${ref_name} | cut -f1)

    # check for empty commit hash
    if [ -z "${commit}" ]; then 
        echo Failed to get last commit hash for ${1}
        exit 1
    fi

    file=${commit}.tgz
    s3_file=${s3_bucket_root}${1}/${OS}/${platform}/${file}

    # Use AWS CLI to download the package corresponding to the component from S3 bucket
    if aws s3 cp --only-show-errors "${s3_file}" './'"${file}" 2>/dev/null; then
        echo Successfully downloaded: ${s3_file}
    else
        echo Failed to get package: ${s3_file} from AWS S3
        exit 1
    fi

    if tar -xzf "${file}" 2>/dev/null; then
        echo Successfully unzipped file:${file}. Now deleting
        rm -f ${file}
    else
        echo Failed to unzip package: ${file}
        exit 1   
    fi
}

# Main script processing the workload. Following steps are performed
# 1. Get the OS to find out if we should build for Darwin(Mac) or Linux
# 2. Check if a Version Input is passed. If not, get current date/time to create a version stamp
# 3. Check if a tag or head name was passed, otherwise use master
# 4. Verify dependencies such as npm, tar and aws cli are installed
# 5. Process the generated packages for each component (pulumi, pulumi-aws, pulumi-cloud) based on the last 
#    commit sha from master branch and extract them to the temporary folder
# 6. run npm for each component
# 7. Build the tar file and upload to S3 bucket

platform=amd64
s3_bucket_root="s3://eng.pulumi.com/releases/"
s3_publish_folder="${s3_bucket_root}sdk/"
components=("pulumi" "pulumi-aws" "pulumi-azure" "pulumi-kubernetes" "pulumi-cloud")
dependencies=("npm" "tar" "aws")

# Get the OS version
OS=""
case $(uname) in
    "Linux") OS="linux";;
    "Darwin") OS="darwin";;
    *) echo "error: unknown host os $(uname)" ; exit 1;;
esac

sdkfilename=pulumi-${1:-$(date +"%Y%m%d_%H%M%S")}-${OS}.x64.tar.gz
ref_name="${2:-master}"

echo Upon completion package: ${sdkfilename} will be uploaded to S3 location: ${s3_publish_folder}

# Verify the required dependencies are already installed
for dependency in ${dependencies[@]}
do
    if ! command -v ${dependency} >/dev/null; then
        echo "required dependency '${dependency}' is not installed"
        exit 1
    fi
done

# setup temporary folder to process the package
pulumi_folder=$(mktemp -d)/pulumi
mkdir -p ${pulumi_folder}

echo ${pulumi_folder}
cd ${pulumi_folder}

cp ${SCRIPT_DIR}/../dist/install.sh .

# Process each component from the components list
for component in ${components[@]}
do
    echo Processing Component: $component
    process_package $component
done

echo npm install pulumi and its components
if [ -d "${pulumi_folder}/node_modules/pulumi" ]; then
    npm_install "${pulumi_folder}/node_modules/pulumi"
fi

if [ -d "${pulumi_folder}/node_modules/@pulumi" ]; then
    for packageDir in "${pulumi_folder}/node_modules/@pulumi"/*; do
        npm_install "$packageDir"
    done
fi

echo zip contents of the folder in a package
sdk_package_path=$(mktemp)

cd ..

if tar -zcf ${sdk_package_path} pulumi; then
    echo Successfully created the package: ${sdk_package_path}
else
    echo Failed to zip the contents of the folder to file: ${sdk_package_path}
    exit 1
fi

echo Upload ${sdk_package_path} to aws s3 bucket ${s3_publish_folder}${sdkfilename}
aws s3 cp --only-show-errors ${sdk_package_path} ${s3_publish_folder}${sdkfilename}

echo Successfully created: ${s3_publish_folder}${sdkfilename}

echo Deleting Temporary folder
rm ${sdk_package_path}
rm -rf  ${pulumi_folder}/

echo Success.
