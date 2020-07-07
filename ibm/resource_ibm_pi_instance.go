package ibm

import (
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/IBM-Cloud/bluemix-go/bmxerror"
	st "github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/helpers"
	"github.com/IBM-Cloud/power-go-client/power/client/p_cloud_p_vm_instances"
	"github.com/IBM-Cloud/power-go-client/power/models"
)

func resourceIBMPIInstance() *schema.Resource {
	return &schema.Resource{
		Create:   resourceIBMPIInstanceCreate,
		Read:     resourceIBMPIInstanceRead,
		Update:   resourceIBMPIInstanceUpdate,
		Delete:   resourceIBMPIInstanceDelete,
		Exists:   resourceIBMPIInstanceExists,
		Importer: &schema.ResourceImporter{},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Minute),
			Update: schema.DefaultTimeout(60 * time.Minute),
			Delete: schema.DefaultTimeout(60 * time.Minute),
		},

		Schema: map[string]*schema.Schema{

			helpers.PICloudInstanceId: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "This is the Power Instance id that is assigned to the account",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "PI instance status",
			},
			"migratable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "set to true to enable migration of the PI instance",
			},
			"min_processors": {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Minimum number of the CPUs",
			},
			"min_memory": {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Minimum memory",
			},
			"max_processors": {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Maximum number of processors",
			},
			"max_memory": {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Maximum memory size",
			},
			helpers.PIInstanceNetworkIds: {
				Type:             schema.TypeSet,
				Required:         true,
				Elem:             &schema.Schema{Type: schema.TypeString},
				Set:              schema.HashString,
				Description:      "Set of Networks that have been configured for the account",
				DiffSuppressFunc: applyOnce,
			},

			helpers.PIInstanceVolumeIds: {
				Type:             schema.TypeSet,
				Optional:         true,
				Computed:         true,
				Elem:             &schema.Schema{Type: schema.TypeString},
				Set:              schema.HashString,
				DiffSuppressFunc: applyOnce,
				Description:      "List of PI volumes",
			},

			helpers.PIInstanceUserData: {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Base64 encoded data to be passed in for invoking a cloud init script",
			},

			"addresses": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"ip": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"macaddress": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"network_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"network_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"external_ip": {
							Type:     schema.TypeString,
							Computed: true,
						},
						/*"version": {
							Type:     schema.TypeFloat,
							Computed: true,
						},*/
					},
				},
			},

			"health_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "PI Instance health status",
			},
			"instance_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Instance ID",
			},
			"pin_policy": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "PIN Policy of the Instance",
			},
			helpers.PIInstanceImageName: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "PI instance image name",
			},
			helpers.PIInstanceProcessors: {
				Type:        schema.TypeFloat,
				Required:    true,
				Description: "Processors count",
			},
			helpers.PIInstanceName: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "PI Instance name",
			},
			helpers.PIInstanceProcType: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateAllowedStringValue([]string{"dedicated", "shared", "capped"}),
				Description:  "Instance processor type",
			},
			helpers.PIInstanceSSHKeyName: {
				Type:        schema.TypeString,
				Required:    true,
				Description: "SSH key name",
			},
			helpers.PIInstanceMemory: {
				Type:        schema.TypeFloat,
				Required:    true,
				Description: "Memory size",
			},
			helpers.PIInstanceSystemType: {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateAllowedStringValue([]string{"any", "s922", "e880", "e980"}),
				Description:  "PI Instance system type",
			},
			helpers.PIInstanceReplicants: {
				Type:        schema.TypeFloat,
				Optional:    true,
				Default:     "1",
				Description: "PI Instance repicas count",
			},
			helpers.PIInstanceReplicationPolicy: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateAllowedStringValue([]string{"affinity", "anti-affinity", "none"}),
				Default:      "none",
				Description:  "Replication policy for the PI INstance",
			},
			helpers.PIInstanceReplicationScheme: {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateAllowedStringValue([]string{"prefix", "suffix"}),
				Default:      "suffix",
				Description:  "Replication scheme",
			},
			helpers.PIInstanceProgress: {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Progress of the operation",
			},
			helpers.PIInstancePinPolicy: {
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Pin Policy of the instance",
				Default:      "none",
				ValidateFunc: validateAllowedStringValue([]string{"none", "soft", "hard"}),
			},

			"reboot_for_resource_change": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Flag to be passed for CPU/Memory changes that require a reboot to take effect",
			},
		},
	}
}

func resourceIBMPIInstanceCreate(d *schema.ResourceData, meta interface{}) error {
	log.Printf("Now in the PowerVMCreate")
	sess, err := meta.(ClientSession).IBMPISession()
	if err != nil {
		return err
	}
	powerinstanceid := d.Get(helpers.PICloudInstanceId).(string)

	name := d.Get(helpers.PIInstanceName).(string)
	sshkey := d.Get(helpers.PIInstanceSSHKeyName).(string)
	mem := d.Get(helpers.PIInstanceMemory).(float64)
	procs := d.Get(helpers.PIInstanceProcessors).(float64)
	systype := d.Get(helpers.PIInstanceSystemType).(string)
	networks := expandStringList((d.Get(helpers.PIInstanceNetworkIds).(*schema.Set)).List())
	volids := expandStringList((d.Get(helpers.PIInstanceVolumeIds).(*schema.Set)).List())
	replicants := d.Get(helpers.PIInstanceReplicants).(float64)
	replicationpolicy := d.Get(helpers.PIInstanceReplicationPolicy).(string)
	replicationNamingScheme := d.Get(helpers.PIInstanceReplicationScheme).(string)
	imageid := d.Get(helpers.PIInstanceImageName).(string)
	processortype := d.Get(helpers.PIInstanceProcType).(string)
	pinpolicy := d.Get(helpers.PIInstancePinPolicy).(string)
	if d.Get(helpers.PIInstancePinPolicy) == "" {
		pinpolicy = "none"
	}

	//var userdata = ""
	user_data := d.Get(helpers.PIInstanceUserData).(string)

	if d.Get(helpers.PIInstanceUserData) == "" {
		user_data = ""
	}
	err = checkBase64(user_data)
	if err != nil {
		log.Printf("Data is not base64 encoded")
		return err
	}

	//publicinterface := d.Get(helpers.PIInstancePublicNetwork).(bool)
	body := &models.PVMInstanceCreate{
		NetworkIds: networks, Processors: &procs, Memory: &mem, ServerName: ptrToString(name),
		SysType:                 systype,
		KeyPairName:             sshkey,
		ImageID:                 ptrToString(imageid),
		ProcType:                ptrToString(processortype),
		Replicants:              replicants,
		UserData:                user_data,
		ReplicantNamingScheme:   ptrToString(replicationNamingScheme),
		ReplicantAffinityPolicy: ptrToString(replicationpolicy),
	}
	if len(volids) > 0 {
		body.VolumeIds = volids
	}
	if d.Get(helpers.PIInstancePinPolicy) == "soft" || d.Get(helpers.PIInstancePinPolicy) == "hard" {
		body.PinPolicy = models.PinPolicy(pinpolicy)
	}

	client := st.NewIBMPIInstanceClient(sess, powerinstanceid)

	pvm, err := client.Create(&p_cloud_p_vm_instances.PcloudPvminstancesPostParams{
		Body: body,
	}, powerinstanceid)
	//log.Printf("the number of instances is %d", len(*pvm))

	if err != nil {
		return fmt.Errorf("Failed to provision the instance")
	} else {
		log.Printf("Printing the instance info %+v", &pvm)
	}

	var pvminstanceids []string
	if replicants > 1 {
		log.Printf("We are in a multi create mode")
		for i := 0; i < int(replicants); i++ {
			truepvmid := (*pvm)[i].PvmInstanceID
			log.Printf("Printing the instance id %s", *truepvmid)
			pvminstanceids = append(pvminstanceids, fmt.Sprintf("%s", *truepvmid))
			log.Printf("Printing each of the pvminstance ids %s", pvminstanceids)
			d.SetId(fmt.Sprintf("%s/%s", powerinstanceid, *truepvmid))
		}
		d.SetId(strings.Join(pvminstanceids, "/"))
	} else {
		log.Printf("Single Create Mode ")
		truepvmid := (*pvm)[0].PvmInstanceID
		d.SetId(fmt.Sprintf("%s/%s", powerinstanceid, *truepvmid))

		pvminstanceids = append(pvminstanceids, *truepvmid)
		log.Printf("Printing the instance id .. after the create ... %s", *truepvmid)
	}

	log.Printf("the number of pvminstanceids is %d", len(pvminstanceids))
	for ids, _ := range pvminstanceids {

		log.Printf("The pvm instance id is [%s] .Checking for status", pvminstanceids[ids])
		//ids, err = strconv.Atoi(str)
		if err != nil {
			return err
		}

		_, err = isWaitForPIInstanceAvailable(client, pvminstanceids[ids], d.Timeout(schema.TimeoutCreate), powerinstanceid)
		if err != nil {
			return err
		}
	}

	return resourceIBMPIInstanceRead(d, meta)

}

func resourceIBMPIInstanceRead(d *schema.ResourceData, meta interface{}) error {

	log.Printf("Calling the PowerInstance Read code..")

	sess, err := meta.(ClientSession).IBMPISession()
	if err != nil {
		return err
	}
	parts, err := idParts(d.Id())
	if err != nil {
		return err
	}
	powerinstanceid := parts[0]
	powerC := st.NewIBMPIInstanceClient(sess, powerinstanceid)
	powervmdata, err := powerC.Get(parts[1], powerinstanceid)

	if err != nil {
		return err
	}

	d.Set(helpers.PIInstanceMemory, powervmdata.Memory)
	d.Set(helpers.PIInstanceProcessors, powervmdata.Processors)
	d.Set("status", powervmdata.Status)
	d.Set(helpers.PIInstanceProcType, powervmdata.ProcType)
	d.Set("migratable", powervmdata.Migratable)
	d.Set("min_processors", powervmdata.Minproc)
	d.Set(helpers.PIInstanceProgress, powervmdata.Progress)
	d.Set(helpers.PICloudInstanceId, powerinstanceid)
	d.Set("instance_id", powervmdata.PvmInstanceID)
	d.Set(helpers.PIInstanceName, powervmdata.ServerName)
	d.Set(helpers.PIInstanceImageName, powervmdata.ImageID)
	var networks []string
	networks = make([]string, 0)
	if powervmdata.Networks != nil {
		for _, n := range powervmdata.Networks {
			if n != nil {
				networks = append(networks, n.NetworkID)
			}

		}
	}
	d.Set(helpers.PIInstanceNetworkIds, newStringSet(schema.HashString, networks))
	d.Set(helpers.PIInstanceVolumeIds, powervmdata.VolumeIds)
	d.Set(helpers.PIInstanceSystemType, powervmdata.SysType)
	d.Set("min_memory", powervmdata.Minmem)
	d.Set("max_processors", powervmdata.Maxproc)
	d.Set("max_memory", powervmdata.Maxmem)
	d.Set("pin_policy", powervmdata.PinPolicy)

	if powervmdata.Addresses != nil {
		pvmaddress := make([]map[string]interface{}, len(powervmdata.Addresses))
		for i, pvmip := range powervmdata.Addresses {
			log.Printf("Now entering the powervm address space....")

			p := make(map[string]interface{})
			p["ip"] = pvmip.IP
			p["network_name"] = pvmip.NetworkName
			p["network_id"] = pvmip.NetworkID
			p["macaddress"] = pvmip.MacAddress
			p["type"] = pvmip.Type
			p["external_ip"] = pvmip.ExternalIP
			pvmaddress[i] = p
		}
		d.Set("addresses", pvmaddress)

		//log.Printf("Printing the value after the read - this should set it.... %+v", pvmaddress)

	}

	if powervmdata.Health != nil {
		d.Set("health_status", powervmdata.Health.Status)

	}

	return nil

}

func resourceIBMPIInstanceUpdate(d *schema.ResourceData, meta interface{}) error {

	name := d.Get(helpers.PIInstanceName).(string)
	mem := d.Get(helpers.PIInstanceMemory).(float64)
	procs := d.Get(helpers.PIInstanceProcessors).(float64)
	processortype := d.Get(helpers.PIInstanceProcType).(string)

	sess, err := meta.(ClientSession).IBMPISession()
	if err != nil {
		return fmt.Errorf("Failed to get the session from the IBM Cloud Service")
	}
	if d.Get("health_status") == "WARNING" {

		return fmt.Errorf("The operation cannot be performed when the lpar health in the WARNING State")
	}

	parts, err := idParts(d.Id())
	if err != nil {
		return err
	}
	powerinstanceid := parts[0]
	client := st.NewIBMPIInstanceClient(sess, powerinstanceid)

	//if d.HasChange(helpers.PIInstanceName) || d.HasChange(helpers.PIInstanceProcessors) || d.HasChange(helpers.PIInstanceProcType) || d.HasChange(helpers.PIInstancePinPolicy){
	if d.HasChange(helpers.PIInstanceProcType) {

		// Stop the lpar
		processortype := d.Get(helpers.PIInstanceProcType).(string)
		if d.Get("status") == "SHUTOFF" {
			log.Printf("the lpar is in the shutoff state. Nothing to do . Moving on ")
		} else {

			body := &models.PVMInstanceAction{
				//Action: ptrToString("stop"),
				Action: ptrToString("immediate-shutdown"),
			}
			resp, err := client.Action(&p_cloud_p_vm_instances.PcloudPvminstancesActionPostParams{Body: body}, parts[1], powerinstanceid)
			if err != nil {
				log.Printf("Stop Action failed on [%s]", name)
				return err
			}
			log.Printf("Getting the response from the shutdown ... %v", resp)

			_, err = isWaitForPIInstanceStopped(client, parts[1], d.Timeout(schema.TimeoutUpdate), powerinstanceid)
			if err != nil {
				return err
			}
		}

		// Modify

		log.Printf("At this point the lpar should be off. Executing the Processor Update Change")
		updatebody := &models.PVMInstanceUpdate{ProcType: processortype}
		updateresp, err := client.Update(parts[1], powerinstanceid, &p_cloud_p_vm_instances.PcloudPvminstancesPutParams{Body: updatebody})
		if err != nil {
			return err
		}
		log.Printf("Getting the response from the change %s", updateresp.StatusURL)

		// To check if the verify resize operation is complete.. and then it will go to SHUTOFF

		_, err = isWaitForPIInstanceStopped(client, parts[1], d.Timeout(schema.TimeoutUpdate), powerinstanceid)
		if err != nil {
			return err
		}

		// Start

		startbody := &models.PVMInstanceAction{
			Action: ptrToString("start"),
		}
		startresp, err := client.Action(&p_cloud_p_vm_instances.PcloudPvminstancesActionPostParams{Body: startbody}, parts[1], powerinstanceid)
		if err != nil {
			return err
		}

		log.Printf("Getting the response from the start %s", startresp)

		_, err = isWaitForPIInstanceAvailable(client, parts[1], d.Timeout(schema.TimeoutUpdate), powerinstanceid)
		if err != nil {
			return err
		}

	}

	// Start of the change for Memory and Processors

	if d.HasChange(helpers.PIInstanceMemory) || d.HasChange(helpers.PIInstanceProcessors) {
		log.Printf("Checking for cpu / memory change..")

		max_mem_lpar := d.Get("max_memory").(float64)
		max_cpu_lpar := d.Get("max_processors").(float64)
		//log.Printf("the required memory is set to [%d] and current max memory is set to  [%d] ", int(mem), int(max_mem_lpar))

		if mem > max_mem_lpar || procs > max_cpu_lpar {
			log.Printf("Will require a shutdown to perform the change")

		} else {
			log.Printf("max_mem_lpar is set to %f", max_mem_lpar)
			log.Printf("max_cpu_lpar is set to %f", max_cpu_lpar)
		}

		//if d.GetOkExists("reboot_for_resource_change")

		if mem > max_mem_lpar || procs > max_cpu_lpar {

			_, err = performChangeAndReboot(client, parts[1], powerinstanceid, mem, procs)
			//_, err = stopLparForResourceChange(client, parts[1], powerinstanceid)
			if err != nil {
				return fmt.Errorf("Failed to perform the operation for the change")
			}

			log.Printf("Getting the response from the bigger change block")

		} else {
			log.Printf("Memory change is within limits")
			parts, err := idParts(d.Id())
			if err != nil {
				return err
			}
			powerinstanceid := parts[0]

			client := st.NewIBMPIInstanceClient(sess, powerinstanceid)

			body := &models.PVMInstanceUpdate{
				Memory:     mem,
				ProcType:   processortype,
				Processors: procs,
				ServerName: name,
			}

			resp, err := client.Update(parts[1], powerinstanceid, &p_cloud_p_vm_instances.PcloudPvminstancesPutParams{Body: body})
			if err != nil {
				return fmt.Errorf("Failed to update the lpar with the change")
			}
			log.Printf("Getting the response from the bigger change block %s", resp.StatusURL)

			_, err = isWaitForPIInstanceAvailable(client, parts[1], d.Timeout(schema.TimeoutUpdate), powerinstanceid)
			if err != nil {
				return err
			}

		}

	}

	return resourceIBMPIInstanceRead(d, meta)

}

func resourceIBMPIInstanceDelete(d *schema.ResourceData, meta interface{}) error {

	log.Printf("Calling the Instance Delete method")
	sess, _ := meta.(ClientSession).IBMPISession()
	parts, err := idParts(d.Id())
	if err != nil {
		return err
	}
	powerinstanceid := parts[0]
	client := st.NewIBMPIInstanceClient(sess, powerinstanceid)

	log.Printf("Deleting the instance with name/id %s and cloud_instance_id %s", parts[1], powerinstanceid)
	err = client.Delete(parts[1], powerinstanceid)
	if err != nil {
		return err
	}

	_, err = isWaitForPIInstanceDeleted(client, parts[1], d.Timeout(schema.TimeoutDelete), powerinstanceid)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}

// Exists

func resourceIBMPIInstanceExists(d *schema.ResourceData, meta interface{}) (bool, error) {

	log.Printf("Calling the PowerInstance Exists method")
	sess, err := meta.(ClientSession).IBMPISession()
	if err != nil {
		return false, err
	}
	parts, err := idParts(d.Id())
	if err != nil {
		return false, err
	}
	powerinstanceid := parts[0]
	client := st.NewIBMPIInstanceClient(sess, powerinstanceid)

	instance, err := client.Get(parts[1], powerinstanceid)
	if err != nil {
		if apiErr, ok := err.(bmxerror.RequestFailure); ok {
			if apiErr.StatusCode() == 404 {
				return false, nil
			}
		}
		return false, fmt.Errorf("Error communicating with the API: %s", err)
	}

	truepvmid := *instance.PvmInstanceID
	return truepvmid == parts[1], nil
}

func isWaitForPIInstanceDeleted(client *st.IBMPIInstanceClient, id string, timeout time.Duration, powerinstanceid string) (interface{}, error) {

	log.Printf("Waiting for  (%s) to be deleted.", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"retry", helpers.PIInstanceDeleting},
		Target:     []string{helpers.PIInstanceNotFound},
		Refresh:    isPIInstanceDeleteRefreshFunc(client, id, powerinstanceid),
		Delay:      10 * time.Second,
		MinTimeout: 10 * time.Second,
		Timeout:    10 * time.Minute,
	}

	return stateConf.WaitForState()
}

func isPIInstanceDeleteRefreshFunc(client *st.IBMPIInstanceClient, id, powerinstanceid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		pvm, err := client.Get(id, powerinstanceid)
		if err != nil {
			log.Printf("The power vm does not exist")
			return pvm, helpers.PIInstanceNotFound, nil

		}
		return pvm, helpers.PIInstanceNotFound, nil

	}
}

func isWaitForPIInstanceAvailable(client *st.IBMPIInstanceClient, id string, timeout time.Duration, powerinstanceid string) (interface{}, error) {
	log.Printf("Waiting for PIInstance (%s) to be available and active ", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"PENDING", "BUILD", helpers.PIInstanceHealthWarning},
		Target:     []string{"OK", "ACTIVE", helpers.PIInstanceHealthOk},
		Refresh:    isPIInstanceRefreshFunc(client, id, powerinstanceid),
		Delay:      10 * time.Second,
		MinTimeout: 2 * time.Minute,
		Timeout:    60 * time.Minute,
	}

	return stateConf.WaitForState()
}

func isPIInstanceRefreshFunc(client *st.IBMPIInstanceClient, id, powerinstanceid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {

		pvm, err := client.Get(id, powerinstanceid)
		if err != nil {
			return nil, "", err
		}

		//if pvm.Health.Status == helpers.PIInstanceHealthOk {
		if *pvm.Status == helpers.PIInstanceAvailable {
			log.Printf("The health status is now ok")
			return pvm, helpers.PIInstanceAvailable, nil

		}

		return pvm, helpers.PIInstanceBuilding, nil
	}
}

func checkPIActive(vminstance *models.PVMInstance) bool {

	log.Printf("Calling the check vm status function and the health status is %s", vminstance.Health.Status)
	activeStatus := false

	if vminstance.Health.Status == "OK" {
		//if *vminstance.Status == "active" {
		log.Printf(" The status of the vm is now set to what we want it to be %s", vminstance.Health.Status)
		activeStatus = true

	}
	return activeStatus
}

func checkBase64(input string) error {
	fmt.Println("Calling the checkBase64")
	data, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		fmt.Println("error:", err)
		return err
	}
	fmt.Printf("Data is correctly Encoded to Base64 %s", data)
	return err

}

func isWaitForPIInstanceStopped(client *st.IBMPIInstanceClient, id string, timeout time.Duration, powerinstanceid string) (interface{}, error) {
	log.Printf("Waiting for PIInstance (%s) to be stopped and powered off ", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"STOPPING", "RESIZE", "VERIFY_RESIZE", helpers.PIInstanceHealthWarning},
		Target:     []string{"OK", "SHUTOFF"},
		Refresh:    isPIInstanceRefreshFuncOff(client, id, powerinstanceid),
		Delay:      10 * time.Second,
		MinTimeout: 2 * time.Minute, // This is the time that the client will execute to check the status of the request
		Timeout:    30 * time.Minute,
	}

	return stateConf.WaitForState()
}

func isPIInstanceRefreshFuncOff(client *st.IBMPIInstanceClient, id, powerinstanceid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {

		log.Printf("Calling the check Refresh status of the pvm [%s] for cloud instance id [%s ]", id, powerinstanceid)
		pvm, err := client.Get(id, powerinstanceid)
		if err != nil {
			return nil, "", err
		}
		log.Printf("The lpar status with id [ %s] is now %s", id, *pvm.Status)
		//if pvm.Health.Status == helpers.PIInstanceHealthOk {
		if *pvm.Status == "SHUTOFF" && pvm.Health.Status == helpers.PIInstanceHealthOk {
			log.Printf("The lpar is now off")

			return pvm, "SHUTOFF", nil
			//}
		}

		return pvm, "STOPPING", nil
	}
}

func stopLparForResourceChange(client *st.IBMPIInstanceClient, id, powerinstanceid string) (interface{}, error) {
	//TODO

	log.Printf("Callin the stop lpar for Resource Change code ..")
	body := &models.PVMInstanceAction{
		//Action: ptrToString("stop"),
		Action: ptrToString("immediate-shutdown"),
	}
	resp, err := client.Action(&p_cloud_p_vm_instances.PcloudPvminstancesActionPostParams{Body: body}, id, powerinstanceid)
	if err != nil {
		log.Printf("Stop Action failed on [%s]", id)
		return nil, err
	}
	log.Printf("Getting the response from the shutdown ... %v", resp)

	_, err = isWaitForPIInstanceStopped(client, id, 30, powerinstanceid)
	if err != nil {
		return nil, fmt.Errorf("Failed to stop the lpar")
	}

	return nil, err
}

// Start the lpar

func startLparAfterResourceChange(client *st.IBMPIInstanceClient, id, powerinstanceid string) (interface{}, error) {
	//TODO

	log.Printf("Callin the start lpar for Resource Change code ..")
	body := &models.PVMInstanceAction{
		//Action: ptrToString("stop"),
		Action: ptrToString("start"),
	}
	resp, err := client.Action(&p_cloud_p_vm_instances.PcloudPvminstancesActionPostParams{Body: body}, id, powerinstanceid)
	if err != nil {
		return nil, fmt.Errorf("Start Action failed on [%s] %s", id, err)
	}
	log.Printf("Getting the response from the start ... %v", resp)

	_, err = isWaitForPIInstanceAvailable(client, id, 30, powerinstanceid)
	if err != nil {
		return nil, fmt.Errorf("Failed to stop the lpar")
	}

	return nil, err
}

// Stop / Modify / Start only when the lpar is off limits

func performChangeAndReboot(client *st.IBMPIInstanceClient, id, powerinstanceid string, mem, procs float64) (interface{}, error) {
	/*
		These are the steps
		1. Stop the lpar - Check if the lpar is SHUTOFF
		2. Once the lpar is SHUTOFF - Make the cpu / memory change - DUring this time , you can check for RESIZE and VERIFY_RESIZE as the transition states
		3. If the change is successful , the lpar state will be back in SHUTOFF
		4. Once the LPAR state is SHUTOFF , initiate the start again and check for ACTIVE + OK
	*/
	//Execute the stop

	log.Printf("Callin the stop lpar for Resource Change code ..")
	stopbody := &models.PVMInstanceAction{
		//Action: ptrToString("stop"),
		Action: ptrToString("immediate-shutdown"),
	}

	resp, err := client.Action(&p_cloud_p_vm_instances.PcloudPvminstancesActionPostParams{Body: stopbody}, id, powerinstanceid)
	if err != nil {
		log.Printf("Stop Action failed on [%s]", id)
		return nil, err
	}
	log.Printf("Getting the response from the shutdown ... %v", resp)

	_, err = isWaitForPIInstanceStopped(client, id, 30, powerinstanceid)
	if err != nil {
		return nil, fmt.Errorf("Failed to stop the lpar")
	}

	log.Printf("Completed the stop successfully. Initiating the resource change ")

	body := &models.PVMInstanceUpdate{
		Memory: mem,
		//ProcType:   processortype,
		Processors: procs,
		//ServerName: name,
	}

	update_resp, update_err := client.Update(id, powerinstanceid, &p_cloud_p_vm_instances.PcloudPvminstancesPutParams{Body: body})
	if update_err != nil {
		return nil, fmt.Errorf("Failed to update the lpar with the change, %s", update_err)
	}
	if update_resp.ServerName == "" {
		log.Printf("the server name is null...from the update call")
	} else {
		log.Printf("printing the response from the update %s", update_resp.ServerName)
	}

	_, err = isWaitforPIInstanceUpdate(client, id, 30, powerinstanceid)
	if err != nil {
		return nil, fmt.Errorf("Failed to get an update from the Service after the resource change, %s", err)
	}

	// Now we can start the lpar

	log.Printf("Callin the start lpar After the  Resource Change code ..")
	startbody := &models.PVMInstanceAction{
		//Action: ptrToString("stop"),
		Action: ptrToString("start"),
	}
	startresp, starterr := client.Action(&p_cloud_p_vm_instances.PcloudPvminstancesActionPostParams{Body: startbody}, id, powerinstanceid)
	if starterr != nil {
		log.Printf("Start Action failed on [%s]", id)

		return nil, fmt.Errorf("The error from the start is %s", starterr)
	}
	log.Printf("Getting the response from the start ... %v", startresp)

	_, err = isWaitForPIInstanceAvailable(client, id, 30, powerinstanceid)
	if err != nil {
		return nil, fmt.Errorf("Failed to stop the lpar %s", err)
	}

	return nil, err

}

func isWaitforPIInstanceUpdate(client *st.IBMPIInstanceClient, id string, timeout time.Duration, powerinstanceid string) (interface{}, error) {
	log.Printf("Waiting for PIInstance (%s) to be SHUTOFF AFTER THE RESIZE Due to DLPAR Operation ", id)

	stateConf := &resource.StateChangeConf{
		Pending:    []string{"RESIZE", "VERIFY_RESIZE"},
		Target:     []string{"ACTIVE", "SHUTOFF", helpers.PIInstanceHealthOk},
		Refresh:    isPIInstanceShutAfterResourceChange(client, id, powerinstanceid),
		Delay:      10 * time.Second,
		MinTimeout: 5 * time.Minute,
		Timeout:    60 * time.Minute,
	}

	return stateConf.WaitForState()
}

func isPIInstanceShutAfterResourceChange(client *st.IBMPIInstanceClient, id, powerinstanceid string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {

		log.Printf("Calling the check lpar status of the pvm [%s] for cloud instance id [%s ] after the resource change", id, powerinstanceid)
		pvm, err := client.Get(id, powerinstanceid)
		if err != nil {
			return nil, "", err
		}
		log.Printf("The lpar status with id [%s] is now %s", id, *pvm.Status)
		//if pvm.Health.Status == helpers.PIInstanceHealthOk {
		if *pvm.Status == "SHUTOFF" && pvm.Health.Status == helpers.PIInstanceHealthOk {
			log.Printf("The lpar is now off after the resource change...")

			return pvm, "SHUTOFF", nil
			//}
		}

		return pvm, "RESIZE", nil
	}
}
