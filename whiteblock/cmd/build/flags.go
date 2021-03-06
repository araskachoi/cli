package build

import (
	"encoding/base64"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/whiteblock/cli/whiteblock/util"
	"io/ioutil"
	"os"
	"strings"
)

func AddBuildFlagsToCommand(cmd *cobra.Command, isAppend bool) {
	cmd.Flags().IntSliceP("servers", "s", []int{}, "manually choose the server options")
	cmd.Flags().BoolP("yes", "y", false, "Yes to all prompts. Evokes default parameters.")
	cmd.Flags().Bool("debug", false, "Yes to all prompts. Evokes default parameters.")
	cmd.Flags().StringP("blockchain", "b", "", "specify blockchain")
	cmd.Flags().IntP("nodes", "n", 0, "specify number of nodes")
	cmd.Flags().StringSliceP("cpus", "c", []string{"0"}, "specify number of cpus")
	cmd.Flags().StringSliceP("memory", "m", []string{"0"}, "specify memory allocated")
	cmd.Flags().StringP("file", "f", "", "parameters file")
	cmd.Flags().IntP("validators", "v", -1, "set the number of validators")
	cmd.Flags().StringSliceP("image", "i", []string{}, "image tag")
	cmd.Flags().StringToStringP("option", "o", nil, "blockchain specific options")
	cmd.Flags().StringToStringP("env", "e", nil, "set environment variables for the nodes")
	cmd.Flags().StringSliceP("template", "t", nil, "set a custom file template")

	cmd.Flags().String("docker-username", "", "docker auth username")
	cmd.Flags().String("docker-password", "", "docker auth password. Note: this will be stored unencrypted while the build is in progress")
	cmd.Flags().StringSlice("user-ssh-key", []string{}, "add an additional ssh key as authorized for the nodes."+
		" Takes a file containing an ssh public key")

	cmd.Flags().Bool("force-docker-pull", false, "Manually pull the image before the build")
	cmd.Flags().Bool("force-unlock", false, "Forcefully stop and unlock the build process")
	cmd.Flags().Bool("freeze-before-genesis", false, "indicate that the build should freeze before starting the genesis ceremony")
	cmd.Flags().String("dockerfile", "", "build from a dockerfile")
	cmd.Flags().StringSliceP("expose-port-mapping", "p", nil, "expose a port to the outside world -p 0=8545:8546")

	cmd.Flags().String("git-repo", "", "build from a git repo")
	cmd.Flags().String("git-repo-branch", "", "specify the branch to build from in a git repo")
	cmd.Flags().IntSlice("expose-all", []int{}, "expose a port linearly for all nodes")
	//META FLAGS
	if !isAppend {
		cmd.Flags().Int("start-logging-at-block", 0, "specify a later block number to start at")
		cmd.Flags().Int("bound-cpus", -1, "specify number of bound cpus")
	}

}

func HandleFreezeBeforeGenesis(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("freeze-before-genesis") {
		return
	}
	fbg, err := cmd.Flags().GetBool("freeze-before-genesis")
	if err != nil {
		util.PrintErrorFatal(err)
	}
	bconf.Extras["freezeAfterInfrastructure"] = fbg
}

func HandleEnv(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("env") {
		return
	}
	envVars, err := cmd.Flags().GetStringToString("env")
	if err != nil {
		util.PrintErrorFatal(err)
	}

	bconf.Environments = make([]map[string]string, bconf.Nodes)
	for i, _ := range bconf.Environments {
		bconf.Environments[i] = make(map[string]string)
	}
	for k, v := range envVars {
		node, key := processEnvKey(k)
		if node == -1 {
			for i, _ := range bconf.Environments {
				bconf.Environments[i][key] = v
			}
			continue
		}
		bconf.Environments[node][key] = v
	}
}

func HandleOptions(cmd *cobra.Command, args []string, bconf *Config, format [][]string) bool {
	if !cmd.Flags().Changed("option") {
		return false
	}
	givenOptions, err := cmd.Flags().GetStringToString("option")
	if err != nil {
		util.PrintErrorFatal(err)
	}
	bconf.Params = map[string]interface{}{}

	for _, kv := range format {
		name := kv[0]
		key_type := kv[1]

		val, ok := givenOptions[name]
		if !ok {
			continue
		}
		switch key_type {
		case "string":
			//needs to have filtering
			bconf.Params[name] = val
		case "[]string":
			preprocessed := strings.Replace(val, " ", ",", -1)
			bconf.Params[name] = strings.Split(preprocessed, ",")
		case "int":
			bconf.Params[name] = util.CheckAndConvertInt64(val, name)

		case "bool":
			switch val {
			case "true":
				fallthrough
			case "yes":
				bconf.Params[name] = true
			case "false":
				fallthrough
			case "no":
				bconf.Params[name] = false
			}
		}
	}
	return true
}

func HandleForceUnlockFlag(cmd *cobra.Command, args []string, bconf *Config) {

	fbg, err := cmd.Flags().GetBool("force-unlock")
	if err == nil && fbg {
		bconf.Extras["forceUnlock"] = true
	}
}

func HandlePullFlag(cmd *cobra.Command, args []string, bconf *Config) {
	_, ok := bconf.Extras["prebuild"]
	if !ok {
		bconf.Extras["prebuild"] = map[string]interface{}{}
	}
	fbg, err := cmd.Flags().GetBool("force-docker-pull")
	if err == nil && fbg {
		bconf.Extras["prebuild"].(map[string]interface{})["pull"] = true
	}
}

func HandleDockerAuthFlags(cmd *cobra.Command, args []string, bconf *Config) {
	if cmd.Flags().Changed("docker-password") != cmd.Flags().Changed("docker-username") {
		if cmd.Flags().Changed("docker-password") {
			util.PrintErrorFatal("you must also provide --docker-password with --docker-username")
		}
		util.PrintErrorFatal("you must also provide --docker-username with --docker-password")
	}
	if !cmd.Flags().Changed("docker-password") {
		return //The auth flags have not been set
	}

	_, ok := bconf.Extras["prebuild"]
	if !ok {
		bconf.Extras["prebuild"] = map[string]interface{}{}
	}

	bconf.Extras["prebuild"].(map[string]interface{})["auth"] = map[string]string{
		"username": util.GetStringFlagValue(cmd, "docker-username"),
		"password": util.GetStringFlagValue(cmd, "docker-password"),
	}

}

func HandleImageFlag(cmd *cobra.Command, args []string, bconf *Config) {

	imageFlag, err := cmd.Flags().GetStringSlice("image")
	if err != nil {
		util.PrintErrorFatal(err)
	}

	bconf.Images = make([]string, bconf.Nodes)
	images, potentialImage, err := util.UnrollStringSliceToMapIntString(imageFlag, "=")
	if err != nil {
		util.PrintErrorFatal(err)
	}

	if len(potentialImage) > 1 {
		util.PrintErrorFatal("too many default images")
	}
	imgDefault := ""
	if len(potentialImage) == 1 {
		imgDefault = potentialImage[0]
		log.WithFields(log.Fields{"image": imgDefault}).Debug("given default image")
	}

	for i := 0; i < bconf.Nodes; i++ {

		image, exists := images[i]
		if exists {
			log.WithFields(log.Fields{"image": image}).Trace("image exists")
			bconf.Images[i] = determineImage(bconf.Blockchain, image)
		} else {
			bconf.Images[i] = determineImage(bconf.Blockchain, imgDefault)
		}
	}
}

func HandleFilesFlag(cmd *cobra.Command, args []string, bconf *Config) {
	filesFlag, err := cmd.Flags().GetStringSlice("template")
	if err != nil {
		util.PrintErrorFatal(err)
	}
	if filesFlag == nil {
		return
	}

	bconf.Files = make([]map[string]string, bconf.Nodes)
	defaults := map[string]string{}
	for _, tfileIn := range filesFlag {
		tuple := strings.SplitN(tfileIn, ";", 3) //support both delim in future
		if len(tuple) < 3 {
			tmp := strings.Replace(tfileIn, ";", "=", 1)
			tuple = strings.SplitN(tmp, "=", 2)
			if len(tuple) != 2 {
				util.PrintErrorFatal(fmt.Errorf("Invalid argument"))
			}
		}
		for i := range tuple {
			tuple[i] = strings.Trim(tuple[i], " \n\r\t")
		}
		if len(tuple) == 2 {
			data, err := ioutil.ReadFile(tuple[1])
			if err != nil {
				util.PrintErrorFatal(err)
			}
			defaults[tuple[0]] = base64.StdEncoding.EncodeToString(data)
			continue
		}
		data, err := ioutil.ReadFile(tuple[2])
		if err != nil {
			util.PrintErrorFatal(err)
		}
		index := util.CheckAndConvertInt(tuple[0], "node number provided to -t")
		if index < 0 || index >= bconf.Nodes {
			util.PrintErrorFatal(fmt.Errorf("Index is out of range for -t flag"))
		}
		bconf.Files[index] = map[string]string{}
		bconf.Files[index][tuple[1]] = base64.StdEncoding.EncodeToString(data)
	}

	if bconf.Extras == nil {
		bconf.Extras = map[string]interface{}{}
	}
	if _, ok := bconf.Extras["defaults"]; !ok {
		bconf.Extras["defaults"] = map[string]interface{}{}
	}
	bconf.Extras["defaults"].(map[string]interface{})["files"] = defaults
}

func HandleSSHOptions(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("user-ssh-key") { //Don't bother if not specified
		return
	}

	sshPubKeys, err := cmd.Flags().GetStringSlice("user-ssh-key")
	if err != nil {
		util.PrintErrorFatal(err)
	}

	if bconf.Extras == nil {
		bconf.Extras = map[string]interface{}{}
	}
	if _, ok := bconf.Extras["postbuild"]; !ok {
		bconf.Extras["postbuild"] = map[string]interface{}{}
	}
	if _, ok := bconf.Extras["postbuild"].(map[string]interface{})["ssh"]; !ok {
		bconf.Extras["postbuild"].(map[string]interface{})["ssh"] = map[string]interface{}{}
	}
	pubKeys := []string{}
	for _, pubKeyFile := range sshPubKeys {
		data, err := ioutil.ReadFile(pubKeyFile)
		if err != nil {
			util.PrintErrorFatal(err)
		}
		pubKeys = append(pubKeys, string(data))
	}

	bconf.Extras["postbuild"].(map[string]interface{})["ssh"].(map[string]interface{})["pubKeys"] = pubKeys
}

func HandleDockerfile(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("dockerfile") {
		return
	}

	filePath := util.GetStringFlagValue(cmd, "dockerfile")
	if len(filePath) == 0 {
		return
	}
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		util.PrintErrorFatal(err)
	}

	if bconf.Extras == nil {
		bconf.Extras = map[string]interface{}{}
	}

	if _, ok := bconf.Extras["prebuild"]; !ok {
		bconf.Extras["prebuild"] = map[string]interface{}{}
	}
	bconf.Extras["prebuild"].(map[string]interface{})["build"] = true
	bconf.Extras["prebuild"].(map[string]interface{})["dockerfile"] = base64.StdEncoding.EncodeToString(data)
}

func HandleStartLoggingAtBlock(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("start-logging-at-block") { //Don't bother if not specified
		return
	}
	bconf.Meta["startBlock"] = util.GetIntFlagValue(cmd, "start-logging-at-block")
}

func HandleResources(cmd *cobra.Command, args []string, bconf *Config) (givenCPU bool, givenMem bool) {
	givenCPU = cmd.Flags().Changed("cpus")
	givenMem = cmd.Flags().Changed("memory")

	if len(bconf.Resources) < bconf.Nodes {
		bconf.Resources = make([]Resources, bconf.Nodes)
	}

	if givenCPU {
		cpus, err := cmd.Flags().GetStringSlice("cpus")
		if err != nil {
			util.PrintErrorFatal(err)
		}

		explicitCpus, defaultCpu, err := util.UnrollStringSliceToMapIntString(cpus, "=")
		if err != nil {
			util.PrintErrorFatal(err)
		}
		log.Trace(explicitCpus, defaultCpu)

		if len(defaultCpu) > 1 {
			util.PrintErrorFatal("too many default cpus")
		}

		cpuDefault := ""
		if len(defaultCpu) == 1 {
			cpuDefault = defaultCpu[0]
			log.Trace(cpuDefault)
		}

		for i := 0; i < bconf.Nodes; i++ {
			cpu, exists := explicitCpus[i]
			if exists {
				bconf.Resources[i].Cpus = string(cpu)
			} else {
				bconf.Resources[i].Cpus = string(cpuDefault)
			}
		}
	}

	if givenMem {
		memories, err := cmd.Flags().GetStringSlice("memory")
		if err != nil {
			util.PrintErrorFatal(err)
		}

		explicitMems, defaultMem, err := util.UnrollStringSliceToMapIntString(memories, "=")
		if err != nil {
			util.PrintErrorFatal(err)
		}
		log.Trace(explicitMems, defaultMem)

		if len(defaultMem) > 1 {
			util.PrintErrorFatal("too many default memory assignemnts")
		}

		memDefault := ""
		if len(defaultMem) == 1 {
			memDefault = defaultMem[0]
			log.Trace(memDefault)
		}

		for i := 0; i < bconf.Nodes; i++ {
			mem, exists := explicitMems[i]
			if exists {
				bconf.Resources[i].Memory = string(mem)
			} else {
				bconf.Resources[i].Memory = string(memDefault)
			}
		}
	}

	return
}

func HandleRepoBuild(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("git-repo") {
		return
	}
	if bconf.Extras == nil {
		bconf.Extras = map[string]interface{}{}
	}

	if _, ok := bconf.Extras["prebuild"]; !ok {
		bconf.Extras["prebuild"] = map[string]interface{}{}
	}
	bconf.Extras["prebuild"].(map[string]interface{})["build"] = true

	repo := util.GetStringFlagValue(cmd, "git-repo")

	bconf.Extras["prebuild"].(map[string]interface{})["repo"] = repo
	if cmd.Flags().Changed("git-repo-branch") {
		branch := util.GetStringFlagValue(cmd, "git-repo-branch")
		log.Trace("given a git repo branch")
		bconf.Extras["prebuild"].(map[string]interface{})["branch"] = branch
	}
}

func addPortMapping(portMapping map[int][]string, bconf *Config) {
	firstResources := bconf.Resources[0]
	for bconf.Nodes > len(bconf.Resources) {
		bconf.Resources = append(bconf.Resources, firstResources)
	}
	for node, mappings := range portMapping {
		bconf.Resources[node].Ports = mappings
		log.WithFields(log.Fields{"node": node, "ports": mappings}).Trace("adding the port mapping")
	}
}

func HandlePortMapping(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("expose-port-mapping") {
		return
	}
	portMapping, err := cmd.Flags().GetStringSlice("expose-port-mapping")
	if err != nil {
		util.PrintErrorFatal(err)
	}

	parsedPortMapping, err := util.ParseIntToStringSlice(portMapping)
	if err != nil {
		util.PrintErrorFatal(err)
	}
	addPortMapping(parsedPortMapping, bconf)

}

func HandleExposeAllBuildFlag(cmd *cobra.Command, args []string, bconf *Config, offset int) {
	if !cmd.Flags().Changed("expose-all") {
		return
	}
	portsToExpose, err := cmd.Flags().GetIntSlice("expose-all")
	if err != nil {
		util.PrintErrorFatal(err)
	}

	portMapping := map[int][]string{}
	usedPort := map[int]bool{}
	for i := 0; i < bconf.Nodes; i++ {
		portMapping[i] = []string{}
		for _, portToExpose := range portsToExpose {
			portToBind := portToExpose + i + offset
			_, used := usedPort[portToBind]
			if used {
				util.PrintErrorFatal(
					fmt.Sprintf("would duplicate exposed port %d. Too many nodes to run auto expose", portToExpose))
			}
			portMapping[i] = append(portMapping[i], fmt.Sprintf("%d:%d", portToBind, portToExpose))
		}
	}
	addPortMapping(portMapping, bconf)
}

func HandleServersFlag(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("servers") {
		bconf.Servers = getServer()
		return
	}
	servers, err := cmd.Flags().GetIntSlice("servers")
	if err != nil {
		util.PrintErrorFatal(err)
	}
	bconf.Servers = servers
}

func HandleBoundCPUs(cmd *cobra.Command, args []string, bconf *Config) {
	if !cmd.Flags().Changed("bound-cpus") {
		return
	}
	firstResources := bconf.Resources[0]
	for bconf.Nodes > len(bconf.Resources) {
		bconf.Resources = append(bconf.Resources, firstResources)
	}
	numCPUs := util.GetIntFlagValue(cmd, "bound-cpus")
	cpuNo := 0
	for i := range bconf.Resources {
		bconf.Resources[i].BoundCPUs = []int{}
		for j := 0; j < numCPUs; j++ {
			bconf.Resources[i].BoundCPUs = append(bconf.Resources[i].BoundCPUs, cpuNo)
			cpuNo++
		}
	}
}

func HandleDebugBuild(cmd *cobra.Command, args []string, bconf *Config) {
	if util.GetBoolFlagValue(cmd, "debug") {
		util.Print(*bconf)
		os.Exit(0)
	}
}
