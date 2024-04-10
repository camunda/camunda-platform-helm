function logs() { 
kubectl logs "$(kubectl get pods | grep integration-venom | awk '{print $1}' | tail -n 1)"
}

function apply(){
kustomize build -o build && cat build | kubectl apply -f -
}

function delete(){
cat build | kubectl delete -f -
}

