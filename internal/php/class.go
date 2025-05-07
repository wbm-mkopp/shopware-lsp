package php

func (c *PHPIndex) GetProperty(className string, name string) *PHPProperty {
	class := c.GetClass(className)
	if class == nil {
		return nil
	}

	property, ok := class.Properties[name]
	if !ok {
		// Property not found in this class, check parent
		if class.Parent != "" {
			parentProperty := c.GetProperty(class.Parent, name)
			if parentProperty != nil {
				return parentProperty
			}
		}

		// Property not found
		return nil
	}

	return &property
}

func (c *PHPIndex) GetMethod(className string, name string) *PHPMethod {
	class := c.GetClass(className)
	if class == nil {
		return nil
	}

	method, ok := class.Methods[name]
	if !ok {
		// Method not found in this class, check parent
		if class.Parent != "" {
			parentMethod := c.GetMethod(class.Parent, name)
			if parentMethod != nil {
				return parentMethod
			}
		}

		// Method not found
		return nil
	}

	return &method
}
