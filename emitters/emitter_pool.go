package emitters

import (
	"fmt"
	"resolve/models"
	"sync"
)

// EmitterPoolImpl implements the EmitterPool interface
type EmitterPoolImpl struct {
	emitters map[string]models.Emitter
	mu       sync.RWMutex
}

// NewEmitterPool creates a new emitter pool
func NewEmitterPool() *EmitterPoolImpl {
	return &EmitterPoolImpl{
		emitters: make(map[string]models.Emitter),
	}
}

// AddEmitter adds an emitter to the pool
func (p *EmitterPoolImpl) AddEmitter(emitter models.Emitter) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := emitter.GetID()
	if _, exists := p.emitters[id]; exists {
		return fmt.Errorf("emitter with ID %s already exists", id)
	}

	p.emitters[id] = emitter
	return nil
}

// RemoveEmitter removes an emitter from the pool
func (p *EmitterPoolImpl) RemoveEmitter(emitterID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.emitters[emitterID]; !exists {
		return fmt.Errorf("emitter with ID %s not found", emitterID)
	}

	delete(p.emitters, emitterID)
	return nil
}

// GetEmitter retrieves an emitter by ID
func (p *EmitterPoolImpl) GetEmitter(emitterID string) (models.Emitter, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	emitter, exists := p.emitters[emitterID]
	if !exists {
		return nil, fmt.Errorf("emitter with ID %s not found", emitterID)
	}

	return emitter, nil
}

// GetAllEmitters returns all emitters in the pool
func (p *EmitterPoolImpl) GetAllEmitters() map[string]models.Emitter {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Create a copy to avoid external modification
	emitters := make(map[string]models.Emitter)
	for id, emitter := range p.emitters {
		emitters[id] = emitter
	}

	return emitters
}

// GetEmitterCount returns the number of emitters in the pool
func (p *EmitterPoolImpl) GetEmitterCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.emitters)
}
