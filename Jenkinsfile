@Library('go-micro-ci') _

// Thin Jenkinsfile — logic nằm trong go-micro-pipeline-lib
ciGoMicroService([
  service   : 'order',
  imageRepo : 'minhtri1612/order-service',
  gitopsRepo: 'https://github.com/minhtri1612/go-micro-gitops.git',
  envFile   : 'env/dev.yaml'
])