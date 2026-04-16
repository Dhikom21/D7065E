package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/eislab-cps/buildingsim/pkg/model"
	"github.com/eislab-cps/buildingsim/pkg/store"
)

type SensorHandlers struct {
	Store *store.MemoryStore
}

func (h *SensorHandlers) AddSensor(c *gin.Context) {
	equipmentID := c.Param("id")
	var sen model.Sensor
	if err := c.ShouldBindJSON(&sen); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if sen.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	ok, err := h.Store.AddSensor(equipmentID, &sen)
	if !ok {
		if errors.Is(err, store.ErrSensorAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "sensor '" + sen.ID + "' already exists on equipment '" + equipmentID + "'"})
		} else if errors.Is(err, store.ErrEquipmentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cannot add sensor '" + sen.ID + "' — equipment '" + equipmentID + "' does not exist"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, sen)
}

func (h *SensorHandlers) ListSensors(c *gin.Context) {
	equipmentID := c.Param("id")
	sensors, ok := h.Store.GetSensorsForEquipment(equipmentID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "equipment not found"})
		return
	}
	c.JSON(http.StatusOK, sensors)
}

func (h *SensorHandlers) DeleteSensor(c *gin.Context) {
	sensorID := c.Param("id")
	if !h.Store.DeleteSensor(sensorID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sensor not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *SensorHandlers) SetValue(c *gin.Context) {
	sensorID := c.Param("id")
	var val model.SensorValue
	if err := c.ShouldBindJSON(&val); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.Store.SetSensorValue(sensorID, val) {
		c.JSON(http.StatusNotFound, gin.H{"error": "sensor not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
