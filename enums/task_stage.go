package enums

type TaskStage string

const (
	TaskReceived   TaskStage = "RECEIVED"
	TaskValidating TaskStage = "VALIDATING"
	TaskStarted    TaskStage = "STARTED"
	TaskProcessing TaskStage = "PROCESSING"
	TaskCanceled   TaskStage = "CANCELED"
	TaskFailed     TaskStage = "FAILED"
	TaskProcessed  TaskStage = "PROCESSED"
)
