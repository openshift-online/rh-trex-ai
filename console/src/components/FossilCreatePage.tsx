import * as React from 'react';
import { useHistory } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  PageSection,
  TextInput,
  TextInput,
  TextInput,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const FossilCreatePage: React.FC = () => {
  const history = useHistory();
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [discoveryLocation, setDiscoveryLocation] = React.useState('');
  const [estimatedAge, setEstimatedAge] = React.useState<number>(0);
  const [excavatorName, setExcavatorName] = React.useState('');
  const [fossilType, setFossilType] = React.useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {};
    if (discoveryLocation) body['discovery_location'] = discoveryLocation;
    if (estimatedAge !== 0) body['estimated_age'] = estimatedAge;
    if (excavatorName) body['excavator_name'] = excavatorName;
    if (fossilType) body['fossil_type'] = fossilType;

    try {
      const api = createAPIClient();
      const created = await api.fossils.create(body);
      history.push(`/trex-console/fossils/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/fossils')}
            component="button"
          >
            Fossils
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Fossil</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form onSubmit={handleSubmit} style={{ maxWidth: 600 }}>
          <FormGroup
            label="Discovery Location"
            fieldId="field-discovery_location"
            isRequired={ true}
          >
            <TextInput
              id="field-discovery_location"
              type="text"
              value={ discoveryLocation}
              onChange={(_e, val) => setDiscoveryLocation(val)}
              placeholder="Enter Discovery Location"
              isRequired={ true}
            />
          </FormGroup>
          <FormGroup
            label="Estimated Age"
            fieldId="field-estimated_age"
            isRequired={ false}
          >
            <TextInput
              id="field-estimated_age"
              type="number"
              value={ estimatedAge}
              onChange={(_e, val) => setEstimatedAge(Number(val))}
              placeholder="0"
              isRequired={ false}
            />
          </FormGroup>
          <FormGroup
            label="Excavator Name"
            fieldId="field-excavator_name"
            isRequired={ false}
          >
            <TextInput
              id="field-excavator_name"
              type="text"
              value={ excavatorName}
              onChange={(_e, val) => setExcavatorName(val)}
              placeholder="Enter Excavator Name"
              isRequired={ false}
            />
          </FormGroup>
          <FormGroup
            label="Fossil Type"
            fieldId="field-fossil_type"
            isRequired={ false}
          >
            <TextInput
              id="field-fossil_type"
              type="text"
              value={ fossilType}
              onChange={(_e, val) => setFossilType(val)}
              placeholder="Enter Fossil Type"
              isRequired={ false}
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/fossils')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default FossilCreatePage;
