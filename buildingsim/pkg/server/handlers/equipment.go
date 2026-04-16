package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/eislab-cps/buildingsim/pkg/model"
	"github.com/eislab-cps/buildingsim/pkg/server/websocket"
	"github.com/eislab-cps/buildingsim/pkg/store"
)

type EquipmentHandlers struct {
	Store *store.MemoryStore
	Hub   *websocket.Hub
}

func (h *EquipmentHandlers) Create(c *gin.Context) {
	var eq model.Equipment
	if err := c.ShouldBindJSON(&eq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if eq.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}
	if eq.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if eq.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type is required"})
		return
	}
	if eq.Level == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level is required"})
		return
	}
	if eq.Room == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room is required"})
		return
	}
	if !h.Store.RoomExists(eq.Level, eq.Room) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room '" + eq.Room + "' not found in level '" + eq.Level + "'"})
		return
	}
	if _, exists := h.Store.GetEquipment(eq.ID); exists {
		c.JSON(http.StatusConflict, gin.H{"error": "equipment already exists"})
		return
	}
	h.Store.CreateEquipment(&eq)
	c.JSON(http.StatusCreated, eq)
}

func (h *EquipmentHandlers) List(c *gin.Context) {
	level := c.Query("level")
	room := c.Query("room")
	typ := c.Query("type")
	category := c.Query("category")
	result := h.Store.ListEquipment(level, room, typ, category)
	if result == nil {
		result = []*model.Equipment{}
	}
	c.JSON(http.StatusOK, result)
}

func (h *EquipmentHandlers) Get(c *gin.Context) {
	id := c.Param("id")
	eq, ok := h.Store.GetEquipment(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "equipment not found"})
		return
	}
	c.JSON(http.StatusOK, eq)
}

func (h *EquipmentHandlers) Update(c *gin.Context) {
	id := c.Param("id")
	var eq model.Equipment
	if err := c.ShouldBindJSON(&eq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	eq.ID = id
	if !h.Store.UpdateEquipment(&eq) {
		c.JSON(http.StatusNotFound, gin.H{"error": "equipment not found"})
		return
	}
	c.JSON(http.StatusOK, eq)
}

func (h *EquipmentHandlers) Delete(c *gin.Context) {
	id := c.Param("id")
	if !h.Store.DeleteEquipment(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "equipment not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *EquipmentHandlers) BulkCreate(c *gin.Context) {
	var items []model.Equipment
	if err := c.ShouldBindJSON(&items); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type succeededEntry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	type failedEntry struct {
		Index         int      `json:"index"`
		ID            string   `json:"id,omitempty"`
		Name          string   `json:"name,omitempty"`
		Reason        string   `json:"reason"`
		MissingFields []string `json:"missing_fields"`
	}

	var succeeded []succeededEntry
	var failed []failedEntry

	for i := range items {
		eq := &items[i]
		var missing []string

		if eq.ID == "" {
			missing = append(missing, "id")
		}
		if eq.Name == "" {
			missing = append(missing, "name")
		}
		if eq.Level == "" {
			missing = append(missing, "level")
		}
		if eq.Room == "" {
			missing = append(missing, "room")
		}
		if eq.Type == "" {
			missing = append(missing, "type")
		}

		if len(missing) > 0 {
			failed = append(failed, failedEntry{
				Index:         i,
				ID:            eq.ID,
				Name:          eq.Name,
				Reason:        "missing required fields",
				MissingFields: missing,
			})
			continue
		}

		if !h.Store.RoomExists(eq.Level, eq.Room) {
			failed = append(failed, failedEntry{
				Index:         i,
				ID:            eq.ID,
				Name:          eq.Name,
				Reason:        "room '" + eq.Room + "' not found in level '" + eq.Level + "'",
				MissingFields: []string{},
			})
			continue
		}
		if _, exists := h.Store.GetEquipment(eq.ID); exists {
			failed = append(failed, failedEntry{
				Index:         i,
				ID:            eq.ID,
				Name:          eq.Name,
				Reason:        "equipment already exists",
				MissingFields: []string{},
			})
			continue
		}
		h.Store.CreateEquipment(eq)
		succeeded = append(succeeded, succeededEntry{ID: eq.ID, Name: eq.Name})
	}

	version := h.Store.BumpEquipmentVersion()
	h.Hub.BroadcastToAll(websocket.Message{Type: "equipment", Version: version})
	c.JSON(http.StatusCreated, gin.H{
		"created":  len(succeeded),
		"skipped":  len(failed),
		"total":    len(items),
		"version":  version,
		"created_items": succeeded,
		"failed_items":  failed,
	})
}

func (h *EquipmentHandlers) Notify(c *gin.Context) {
	equipment := h.Store.ListEquipment("", "", "", "")
	if len(equipment) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no equipment registered, nothing to notify"})
		return
	}
	version := h.Store.BumpEquipmentVersion()
	h.Hub.BroadcastToAll(websocket.Message{
		Type:    "equipment",
		Version: version,
	})
	c.JSON(http.StatusOK, gin.H{"version": version, "notified_equipment_count": len(equipment)})
}
