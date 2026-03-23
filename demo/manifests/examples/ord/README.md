# Create port-forwards to the services for ORD

 kubectl port-forward svc/spaceship-app 3001:3000
 kubectl port-forward svc/super-agent 3002:3000
 k port-forward svc/basic-ord 3004:8083