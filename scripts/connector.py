import boto3
import os
import json

dynamodb = boto3.resource('dynamodb', region_name=os.environ.get('DYNAMO_REGION', 'af-south-1'))
table = dynamodb.Table('Connections')

def handler(event, context):
    connection_id = event['requestContext']['connectionId']
    event_type = event['requestContext']['eventType']

    if event_type == 'CONNECT':
        table.put_item(Item={'ConnectionID': connection_id})
        print(f"CONNECT: {connection_id}")
    elif event_type == 'DISCONNECT':
        table.delete_item(Key={'ConnectionID': connection_id})
        print(f"DISCONNECT: {connection_id}")

    return {'statusCode': 200}
