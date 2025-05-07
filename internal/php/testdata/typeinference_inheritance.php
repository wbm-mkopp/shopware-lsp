<?php

namespace App\Entity;

interface ProductInterface
{
    public function getDescription(): string;
    public function getPrice(): float;
}

abstract class BaseProduct implements ProductInterface
{
    protected int $id;
    protected string $description;
    protected float $price;

    public function getId(): int
    {
        return $this->id;
    }

    public function getDescription(): string
    {
        return $this->description;
    }

    public function getBaseInformation(): array
    {
        return [
            'id' => $this->getId(),
            'description' => $this->getDescription()
        ];
    }
}

class Product extends BaseProduct
{
    private string $name;
    private ?string $sku = null;

    public function getName(): string
    {
        return $this->name;
    }

    public function setName(string $name): self
    {
        $this->name = $name;
        return $this;
    }

    public function getPrice(): float
    {
        return $this->price;
    }

    public function getSku(): ?string
    {
        return $this->sku;
    }

    public function getProductData(): array
    {
        // Test inheritance - calls methods from parent and self
        $baseInfo = $this->getBaseInformation();
        $price = $this->getPrice();
        $name = $this->getName();
        $sku = $this->getSku();
        
        return array_merge($baseInfo, [
            'name' => $name,
            'price' => $price,
            'sku' => $sku
        ]);
    }
}
