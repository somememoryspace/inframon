#!/bin/bash

# Get the job name from the yaml file
JOB_NAME=$(grep -E '^\s*name:' job.yaml | head -1 | awk '{print $2}')
NS_NAME=$(grep -E '^\s*namespace:' job.yaml | head -1 | awk '{print $2}')

echo "Job Name: $JOB_NAME"
echo "Namespace Name: $NS_NAME"

# Check if job already exists
if kubectl get job -n $NS_NAME $JOB_NAME &> /dev/null; then
    echo "Job $JOB_NAME in $NS_NAME already exists"
    
    # Check if it's complete
    if kubectl wait --for=condition=complete --timeout=0s -n $NS_NAME job/$JOB_NAME &> /dev/null; then
        echo "Job is complete, deleting..."
        kubectl delete -f job.yaml
    else
        echo "Job exists but is not complete, deleting anyway..."
        kubectl delete -f job.yaml
    fi
    
    # Wait a moment for cleanup
    sleep 2
fi

# Deploy the new job
echo "Deploying new job..."
kubectl apply -f job.yaml

# Wait for the job to complete
echo "Waiting for job to complete..."
kubectl wait --for=condition=complete --timeout=300s -n $NS_NAME job/$JOB_NAME
kubectl logs -n $NS_NAME job/$JOB_NAME

# Delete the job
echo "Deleting job..."
kubectl delete -f job.yaml
echo "Job $JOB_NAME completed and deleted"