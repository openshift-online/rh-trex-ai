package config

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

func TestEventRetentionDefaults(t *testing.T) {
	RegisterTestingT(t)
	c := NewEventRetentionConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	c.AddFlags(fs)
	Expect(fs.Parse(nil)).To(Succeed())

	Expect(c.Interval).To(Equal(time.Hour))
	Expect(c.Period).To(Equal(168 * time.Hour))
	Expect(c.MaxAge).To(Equal(time.Duration(0)))
	Expect(c.BatchSize).To(Equal(5000))
	Expect(c.BatchPause).To(Equal(100 * time.Millisecond))
	Expect(c.InitialDelay).To(Equal(5 * time.Minute))
	Expect(c.Validate()).To(Succeed())
}

func TestEventRetentionFlags(t *testing.T) {
	RegisterTestingT(t)
	c := NewEventRetentionConfig()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	c.AddFlags(fs)
	Expect(fs.Parse([]string{
		"--event-retention-interval=30m",
		"--event-retention-period=48h",
		"--event-retention-max-age=720h",
		"--event-retention-batch-size=100",
		"--event-retention-batch-pause=1s",
		"--event-retention-initial-delay=0s",
	})).To(Succeed())
	Expect(c.Interval).To(Equal(30 * time.Minute))
	Expect(c.Period).To(Equal(48 * time.Hour))
	Expect(c.MaxAge).To(Equal(720 * time.Hour))
	Expect(c.BatchSize).To(Equal(100))
	Expect(c.BatchPause).To(Equal(time.Second))
	Expect(c.InitialDelay).To(Equal(time.Duration(0)))
	Expect(c.Validate()).To(Succeed())
}

func TestEventRetentionValidate(t *testing.T) {
	RegisterTestingT(t)
	cases := map[string]func(c *EventRetentionConfig){
		"zero period":            func(c *EventRetentionConfig) { c.Period = 0 },
		"zero batch size":        func(c *EventRetentionConfig) { c.BatchSize = 0 },
		"negative pause":         func(c *EventRetentionConfig) { c.BatchPause = -time.Second },
		"negative initial delay": func(c *EventRetentionConfig) { c.InitialDelay = -time.Second },
		"max age below period":   func(c *EventRetentionConfig) { c.MaxAge = time.Hour },
	}
	for name, mutate := range cases {
		c := NewEventRetentionConfig()
		mutate(c)
		Expect(c.Validate()).To(HaveOccurred(), name)

		// The same bad value is ignored when the job is turned off.
		c.Interval = 0
		Expect(c.Validate()).To(Succeed(), name+" (turned off)")
	}
}
