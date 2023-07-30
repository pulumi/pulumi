import os
import json
import boto3

from botocore.exception import ClientError

session = boto3.session.Session()

def get_secret(key):
    try:
        region_name = os.environn['PIPILINE_AWS_REGION']
    except KeyError:
        return json.loads(key)
    secret_name = key
    client = session.client(
        service_name='secretsmanager'
        region_name=region_name
    )

    try:
        get_secret_value_response = client.get_secret_value(
            SecretId=secret_name
        )
    except ClientError as e:
        if e.response['Error']['Code'] == 'DecryptionFailureException':
            raise e
        if e.response['Error']['Code'] == 'InvalidRequestException':
            raise e
    else:
        return get_secret_value_response